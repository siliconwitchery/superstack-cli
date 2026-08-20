package commands

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func Login(session Session, arguments []string) error {
	oauthClient := &http.Client{Timeout: 30 * time.Second}

	if len(arguments) != 1 || (arguments[0] != "github" && arguments[0] != "gitlab") {
		return errors.New("login takes a provider: github or gitlab")
	}

	provider := arguments[0]

	// Ask the server which oauth apps to log in against
	providersRequest, err := apiRequest(session, http.MethodGet, "/login", nil)

	if err != nil {
		return err
	}

	providersResponse, err := session.Client.Do(providersRequest)

	if err != nil {
		return fmt.Errorf("the server could not be reached: %w", err)
	}

	defer providersResponse.Body.Close()

	if providersResponse.StatusCode != http.StatusOK {
		return serverError(providersResponse)
	}

	providers := struct {
		GithubClientId string `json:"github_client_id"`
		GitlabClientId string `json:"gitlab_client_id"`
	}{}

	err = json.NewDecoder(providersResponse.Body).Decode(&providers)

	if err != nil {
		return err
	}

	// Pick the provider's endpoints
	var clientId, deviceCodeUrl, pollUrl, scope string

	switch provider {
	case "github":
		clientId = providers.GithubClientId
		deviceCodeUrl = session.GithubBase + "/login/device/code"
		pollUrl = session.GithubBase + "/login/oauth/access_token"
		scope = "user:email"

	case "gitlab":
		clientId = providers.GitlabClientId
		deviceCodeUrl = session.GitlabBase + "/oauth/authorize_device"
		pollUrl = session.GitlabBase + "/oauth/token"
		scope = "read_user"
	}

	if clientId == "" {
		return fmt.Errorf("the server offers no %s login", provider)
	}

	// Ask for a one-time code
	codeForm := url.Values{
		"client_id": {clientId},
		"scope":     {scope},
	}

	codeRequest, err := http.NewRequest(http.MethodPost, deviceCodeUrl,
		strings.NewReader(codeForm.Encode()))

	if err != nil {
		return err
	}

	codeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	codeRequest.Header.Set("Accept", "application/json")

	codeResponse, err := oauthClient.Do(codeRequest)

	if err != nil {
		return fmt.Errorf("%s could not be reached: %w", provider, err)
	}

	defer codeResponse.Body.Close()

	code := struct {
		DeviceCode              string `json:"device_code"`
		UserCode                string `json:"user_code"`
		VerificationUri         string `json:"verification_uri"`
		VerificationUriComplete string `json:"verification_uri_complete"`
		ExpiresIn               int    `json:"expires_in"`
		Interval                int    `json:"interval"`
		Error                   string `json:"error"`
	}{}

	err = json.NewDecoder(codeResponse.Body).Decode(&code)

	if err != nil {
		return fmt.Errorf("%s answered %s to the device code request", provider, codeResponse.Status)
	}

	if code.Error != "" || code.DeviceCode == "" {
		return fmt.Errorf("%s would not start the login: %s", provider, code.Error)
	}

	enterAt := code.VerificationUri

	if code.VerificationUriComplete != "" {
		enterAt = code.VerificationUriComplete
	}

	fmt.Fprintf(session.Out, "Copy your one-time code: %s\n", code.UserCode)
	fmt.Fprintf(session.Out, "Then enter it at %s\n", enterAt)
	fmt.Fprintln(session.Out, "Press enter to open the browser.")

	go func() {
		_, err := bufio.NewReader(session.In).ReadString('\n')

		if err == nil {
			session.OpenBrowser(enterAt)
		}
	}()

	// Poll until the code is entered
	deadline := time.Now().Add(time.Duration(code.ExpiresIn) * time.Second)

	const defaultPollInterval = 5
	interval := code.Interval

	if interval <= 0 {
		interval = defaultPollInterval
	}

	accessToken := ""

	for accessToken == "" {

		time.Sleep(time.Duration(interval) * time.Second)

		if time.Now().After(deadline) {
			return errors.New("the code expired before it was entered, run login again")
		}

		pollForm := url.Values{
			"client_id":   {clientId},
			"device_code": {code.DeviceCode},
			"grant_type":  {"urn:ietf:params:oauth:grant-type:device_code"},
		}

		pollRequest, err := http.NewRequest(http.MethodPost, pollUrl,
			strings.NewReader(pollForm.Encode()))

		if err != nil {
			return err
		}

		pollRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		pollRequest.Header.Set("Accept", "application/json")

		pollResponse, err := oauthClient.Do(pollRequest)

		if err != nil {
			return fmt.Errorf("%s could not be reached: %w", provider, err)
		}

		poll := struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
		}{}

		err = json.NewDecoder(pollResponse.Body).Decode(&poll)

		pollResponse.Body.Close()

		if err != nil {
			return fmt.Errorf("%s answered %s while polling", provider, pollResponse.Status)
		}

		switch poll.Error {
		case "":
			if poll.AccessToken == "" {
				return fmt.Errorf("%s approved the login but it did not complete", provider)
			}

			accessToken = poll.AccessToken

		case "authorization_pending":

		case "slow_down":
			interval += 5

		case "expired_token":
			return errors.New("the code expired before it was entered, run login again")

		case "access_denied":
			return fmt.Errorf("the login was declined on %s", provider)

		default:
			return fmt.Errorf("%s answered %q while polling", provider, poll.Error)
		}
	}

	// Trade the provider's token for a superstack key
	loginBody, err := json.Marshal(map[string]string{
		"provider":     provider,
		"access_token": accessToken,
	})

	if err != nil {
		return err
	}

	loginRequest, err := apiRequest(session, http.MethodPost, "/login", bytes.NewReader(loginBody))

	if err != nil {
		return err
	}

	loginRequest.Header.Set("Content-Type", "application/json")

	loginResponse, err := session.Client.Do(loginRequest)

	if err != nil {
		return fmt.Errorf("the server could not be reached: %w", err)
	}

	defer loginResponse.Body.Close()

	if loginResponse.StatusCode != http.StatusOK {
		return serverError(loginResponse)
	}

	login := struct {
		Key   string `json:"key"`
		Email string `json:"email"`
	}{}

	err = json.NewDecoder(loginResponse.Body).Decode(&login)

	if err != nil {
		return err
	}

	if login.Key == "" {
		return errors.New("the login did not complete")
	}

	// Store the key
	path, err := keyPath()

	if err != nil {
		return err
	}

	err = os.MkdirAll(filepath.Dir(path), 0o700)

	if err != nil {
		return err
	}

	err = os.WriteFile(path, []byte(login.Key+"\n"), 0o600)

	if err != nil {
		return err
	}

	fmt.Fprintf(session.Out, "Logged in as %s.\n", login.Email)

	return nil
}
