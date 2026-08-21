package login

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/siliconwitchery/superstack-cli/internal/api"
)

func Login(session api.Session, arguments []string) error {
	oauthClient := &http.Client{Timeout: 30 * time.Second}

	if len(arguments) != 1 || (arguments[0] != "github" && arguments[0] != "gitlab") {
		return errors.New("login takes a provider: github or gitlab")
	}

	provider := arguments[0]

	providersRequest, err := api.Request(session, http.MethodGet, "/login", nil)

	if err != nil {
		return err
	}

	providersResponse, err := session.Client.Do(providersRequest)

	if err != nil {
		return errors.New("the server could not be reached, check your connection")
	}

	defer providersResponse.Body.Close()

	if providersResponse.StatusCode != http.StatusOK {
		return api.ServerError(providersResponse)
	}

	providers := struct {
		GithubClientId string `json:"github_client_id"`
		GitlabClientId string `json:"gitlab_client_id"`
	}{}

	err = api.Decode(providersResponse, &providers)

	if err != nil {
		return err
	}

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
		return fmt.Errorf("%s could not be reached, check your connection", provider)
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

	err = api.Decode(codeResponse, &code)

	if err != nil {
		return fmt.Errorf("%s would not start the login, try again", provider)
	}

	if code.Error != "" || code.DeviceCode == "" {
		return fmt.Errorf("%s would not start the login, try again", provider)
	}

	enterAt := code.VerificationUri

	if code.VerificationUriComplete != "" {
		enterAt = code.VerificationUriComplete
	}

	fmt.Fprintf(session.Out, "Copy your one-time code: %s\n", api.Printable(code.UserCode))
	fmt.Fprintf(session.Out, "Then enter it at %s\n", api.Printable(enterAt))
	fmt.Fprintln(session.Out, "Press enter to open the browser.")

	go func() {
		_, err := bufio.NewReader(session.In).ReadString('\n')

		if err == nil {
			session.OpenBrowser(enterAt)
		}
	}()

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
			return fmt.Errorf("%s could not be reached, check your connection", provider)
		}

		poll := struct {
			AccessToken string `json:"access_token"`
			Error       string `json:"error"`
		}{}

		err = api.Decode(pollResponse, &poll)

		pollResponse.Body.Close()

		if err != nil {
			return fmt.Errorf("%s stopped answering, run login again", provider)
		}

		switch poll.Error {
		case "":
			if poll.AccessToken == "" {
				return fmt.Errorf("the login did not complete on %s, run login again", provider)
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
			return fmt.Errorf("the login did not complete on %s, run login again", provider)
		}
	}

	loginBody, err := json.Marshal(map[string]string{
		"provider":     provider,
		"access_token": accessToken,
	})

	if err != nil {
		return err
	}

	loginRequest, err := api.Request(session, http.MethodPost, "/login", bytes.NewReader(loginBody))

	if err != nil {
		return err
	}

	loginRequest.Header.Set("Content-Type", "application/json")

	loginResponse, err := session.Client.Do(loginRequest)

	if err != nil {
		return errors.New("the server could not be reached, check your connection")
	}

	defer loginResponse.Body.Close()

	if loginResponse.StatusCode != http.StatusOK {
		return api.ServerError(loginResponse)
	}

	login := struct {
		Key   string `json:"key"`
		Email string `json:"email"`
	}{}

	err = api.Decode(loginResponse, &login)

	if err != nil {
		return err
	}

	if login.Key == "" {
		return errors.New("the login did not complete")
	}

	path, err := api.KeyPath()

	if err != nil {
		return err
	}

	directory := filepath.Dir(path)

	err = os.MkdirAll(directory, 0o700)

	if err != nil {
		return fmt.Errorf("the login could not be saved to %s, so you are not logged in", path)
	}

	temporary, err := os.CreateTemp(directory, "key")

	if err != nil {
		return fmt.Errorf("the login could not be saved to %s, so you are not logged in", path)
	}

	defer os.Remove(temporary.Name())

	_, err = temporary.WriteString(login.Key + "\n")

	if err != nil {
		temporary.Close()
		return fmt.Errorf("the login could not be saved to %s, so you are not logged in", path)
	}

	err = temporary.Close()

	if err != nil {
		return fmt.Errorf("the login could not be saved to %s, so you are not logged in", path)
	}

	err = os.Rename(temporary.Name(), path)

	if err != nil {
		return fmt.Errorf("the login could not be saved to %s, so you are not logged in", path)
	}

	fmt.Fprintf(session.Out, "Logged in as %s.\n", api.Printable(login.Email))

	return nil
}

func Logout(session api.Session, arguments []string) error {
	if len(arguments) != 0 {
		return errors.New("logout takes no arguments")
	}

	path, err := api.KeyPath()

	if err != nil {
		return err
	}

	keyBytes, err := os.ReadFile(path)

	if errors.Is(err, fs.ErrNotExist) {
		fmt.Fprintln(session.Out, "Not logged in.")
		return nil
	}

	if err != nil {
		return errors.New("the login stored on this computer could not be read")
	}

	key := strings.TrimSpace(string(keyBytes))

	if key == "" {
		fmt.Fprintln(session.Out, "Not logged in.")
		return nil
	}

	revokeRequest, err := api.Request(session, http.MethodPost, "/logout", nil)

	if err != nil {
		return err
	}

	revokeRequest.Header.Set("Authorization", "Bearer "+key)

	revokeResponse, err := session.Client.Do(revokeRequest)

	if err != nil {
		return errors.New("you are still logged in, the server could not be reached")
	}

	defer revokeResponse.Body.Close()

	if revokeResponse.StatusCode != http.StatusNoContent {
		return fmt.Errorf("you are still logged in: %s", api.ServerError(revokeResponse))
	}

	fmt.Fprintln(session.Out, "Logged out.")

	err = os.Remove(path)

	if err != nil {
		return fmt.Errorf("the login stored at %s is no longer valid but could not be removed, delete it yourself", path)
	}

	return nil
}
