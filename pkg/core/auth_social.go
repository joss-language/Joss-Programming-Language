package core

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Supported OAuth Providers:
// "google", "apple", "facebook", "x" (or "twitter"), "github", "twitch", "yahoo", "microsoft"

type SocialUserInfo struct {
	Provider       string
	ProviderUserID string
	Email          string
	Name           string
	FirstName      string
	LastName       string
	Avatar         string
}

// providerEnvKeys maps provider name to environment variable keys for client_id and client_secret
func getProviderEnvKeys(provider string) (idKey, secretKey string) {
	norm := strings.ToLower(strings.TrimSpace(provider))
	switch norm {
	case "google":
		return "GOOGLE_CLIENT_ID", "GOOGLE_CLIENT_SECRET"
	case "apple":
		return "APPLE_CLIENT_ID", "APPLE_CLIENT_SECRET"
	case "facebook":
		return "FACEBOOK_CLIENT_ID", "FACEBOOK_CLIENT_SECRET"
	case "x", "twitter":
		return "X_CLIENT_ID", "X_CLIENT_SECRET"
	case "github":
		return "GITHUB_CLIENT_ID", "GITHUB_CLIENT_SECRET"
	case "twitch":
		return "TWITCH_CLIENT_ID", "TWITCH_CLIENT_SECRET"
	case "yahoo":
		return "YAHOO_CLIENT_ID", "YAHOO_CLIENT_SECRET"
	case "microsoft":
		return "MICROSOFT_CLIENT_ID", "MICROSOFT_CLIENT_SECRET"
	default:
		upper := strings.ToUpper(norm)
		return upper + "_CLIENT_ID", upper + "_CLIENT_SECRET"
	}
}

// EnabledSocialProviders returns the list of social providers that have their credentials set in Env
func (r *Runtime) EnabledSocialProviders() []interface{} {
	providers := []string{"google", "apple", "facebook", "x", "github", "twitch", "yahoo", "microsoft"}
	var enabled []interface{}
	for _, p := range providers {
		idKey, secretKey := getProviderEnvKeys(p)
		clientId := r.getEnvOrOs(idKey)
		clientSecret := r.getEnvOrOs(secretKey)
		// For X/Twitter, also check TWITTER_CLIENT_ID if X_CLIENT_ID is not set
		if p == "x" && clientId == "" {
			clientId = r.getEnvOrOs("TWITTER_CLIENT_ID")
			clientSecret = r.getEnvOrOs("TWITTER_CLIENT_SECRET")
		}
		if clientId != "" && clientSecret != "" {
			enabled = append(enabled, p)
		}
	}
	return enabled
}

func (r *Runtime) getEnvOrOs(key string) string {
	if val, ok := r.Env[key]; ok && strings.TrimSpace(val) != "" {
		return strings.TrimSpace(val)
	}
	return ""
}

// GenerateSocialAuthURL builds the authorization redirect URL for the given provider
func (r *Runtime) GenerateSocialAuthURL(provider, redirectURI, state string) (string, error) {
	norm := strings.ToLower(strings.TrimSpace(provider))
	idKey, _ := getProviderEnvKeys(norm)
	clientID := r.getEnvOrOs(idKey)
	if norm == "x" && clientID == "" {
		clientID = r.getEnvOrOs("TWITTER_CLIENT_ID")
	}
	if clientID == "" {
		return "", fmt.Errorf("Social auth error: %s credentials (client_id) not configured", norm)
	}

	if state == "" {
		state = generateRandomState()
	}

	var authBase string
	var scope string
	var extraParams = url.Values{}

	switch norm {
	case "google":
		authBase = "https://accounts.google.com/o/oauth2/v2/auth"
		scope = "openid email profile"
		extraParams.Set("access_type", "offline")
		extraParams.Set("prompt", "select_account")

	case "github":
		authBase = "https://github.com/login/oauth/authorize"
		scope = "read:user user:email"

	case "microsoft":
		authBase = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
		scope = "openid email profile User.Read"
		extraParams.Set("response_mode", "query")

	case "facebook":
		authBase = "https://www.facebook.com/v19.0/dialog/oauth"
		scope = "email,public_profile"

	case "apple":
		authBase = "https://appleid.apple.com/auth/authorize"
		scope = "name email"
		extraParams.Set("response_mode", "form_post")

	case "x", "twitter":
		authBase = "https://twitter.com/i/oauth2/authorize"
		scope = "tweet.read users.read"
		extraParams.Set("code_challenge", "challenge")
		extraParams.Set("code_challenge_method", "plain")

	case "twitch":
		authBase = "https://id.twitch.tv/oauth2/authorize"
		scope = "user:read:email"

	case "yahoo":
		authBase = "https://api.login.yahoo.com/oauth2/request_auth"
		scope = "openid profile email"

	default:
		return "", fmt.Errorf("Social auth provider %s is not supported", norm)
	}

	v := url.Values{}
	v.Set("client_id", clientID)
	v.Set("redirect_uri", redirectURI)
	v.Set("response_type", "code")
	v.Set("scope", scope)
	v.Set("state", state)

	for key, values := range extraParams {
		for _, val := range values {
			v.Set(key, val)
		}
	}

	return fmt.Sprintf("%s?%s", authBase, v.Encode()), nil
}

// HandleSocialCallback exchanges the authorization code for tokens, retrieves profile, and provisions/links user
func (r *Runtime) HandleSocialCallback(provider, code, redirectURI string) (*Instance, error) {
	norm := strings.ToLower(strings.TrimSpace(provider))
	idKey, secKey := getProviderEnvKeys(norm)
	clientID := r.getEnvOrOs(idKey)
	clientSecret := r.getEnvOrOs(secKey)
	if norm == "x" && clientID == "" {
		clientID = r.getEnvOrOs("TWITTER_CLIENT_ID")
		clientSecret = r.getEnvOrOs("TWITTER_CLIENT_SECRET")
	}

	if clientID == "" || clientSecret == "" {
		return nil, fmt.Errorf("Social auth credentials not configured for %s", norm)
	}

	userInfo, err := r.exchangeCodeAndFetchUser(norm, clientID, clientSecret, code, redirectURI)
	if err != nil {
		return nil, fmt.Errorf("OAuth callback failed for %s: %w", norm, err)
	}

	if userInfo.ProviderUserID == "" {
		return nil, fmt.Errorf("OAuth error: could not obtain user ID from %s", norm)
	}

	prefix := r.dbPrefix()
	usersTable := prefix + "users"
	socialTable := prefix + "user_social_accounts"
	r.ensureAuthTables(usersTable, prefix+"roles", prefix)

	// Check if a user is currently logged in (linking flow from profile or settings)
	loggedInUserId := r.currentSessionUserId()

	// Check if this social account is already linked
	var existingUserId int
	checkSocialQuery := fmt.Sprintf("SELECT user_id FROM %s WHERE provider = ? AND provider_user_id = ? LIMIT 1", socialTable)
	err = r.databaseExecutor().QueryRow(checkSocialQuery, norm, userInfo.ProviderUserID).Scan(&existingUserId)

	if err == nil && existingUserId > 0 {
		// Existing user found by social account
		if loggedInUserId > 0 && existingUserId != loggedInUserId {
			return nil, fmt.Errorf("Esta cuenta de %s ya está vinculada a otro usuario.", norm)
		}
		return r.createAuthLoginResultForUser(existingUserId)
	}

	// If the user is currently logged in, link the social account directly to their account
	// (regardless of whether the social email matches the user email)
	if loggedInUserId > 0 {
		insertSocial := map[string]interface{}{
			"user_id":          loggedInUserId,
			"provider":         norm,
			"provider_user_id": userInfo.ProviderUserID,
			"avatar":           userInfo.Avatar,
		}
		r.insertFromMap(socialTable, insertSocial, false)
		return r.createAuthLoginResultForUser(loggedInUserId)
	}

	// Not logged in and not linked yet. Check if a user with the same email already exists
	if userInfo.Email != "" {
		var emailUserId int
		checkEmailQuery := fmt.Sprintf("SELECT id FROM %s WHERE email = ? LIMIT 1", usersTable)
		err = r.databaseExecutor().QueryRow(checkEmailQuery, userInfo.Email).Scan(&emailUserId)
		if err == nil && emailUserId > 0 {
			// Link social account to existing user
			insertSocial := map[string]interface{}{
				"user_id":          emailUserId,
				"provider":         norm,
				"provider_user_id": userInfo.ProviderUserID,
				"avatar":           userInfo.Avatar,
			}
			r.insertFromMap(socialTable, insertSocial, false)
			return r.createAuthLoginResultForUser(emailUserId)
		}
	}

	// Brand new user registration via social login
	userToken := uuid.New().String()
	randomPwdBytes := make([]byte, 24)
	rand.Read(randomPwdBytes)
	hashedPwd, _ := bcrypt.GenerateFromPassword([]byte(base64.URLEncoding.EncodeToString(randomPwdBytes)), bcrypt.DefaultCost)

	firstName := userInfo.FirstName
	lastName := userInfo.LastName
	if firstName == "" && userInfo.Name != "" {
		parts := strings.SplitN(userInfo.Name, " ", 2)
		firstName = parts[0]
		if len(parts) > 1 {
			lastName = parts[1]
		}
	}
	if firstName == "" {
		firstName = fmt.Sprintf("User_%s", norm)
	}

	userEmail := userInfo.Email
	if userEmail == "" {
		userEmail = fmt.Sprintf("%s_%s@social.joss.local", norm, userInfo.ProviderUserID)
	}

	username := strings.ToLower(strings.ReplaceAll(firstName, " ", ""))
	if len(username) > 20 {
		username = username[:20]
	}
	username = fmt.Sprintf("%s_%d", username, time.Now().Unix()%10000)

	newUserData := map[string]interface{}{
		"user_token":       userToken,
		"username":         username,
		"first_name":       firstName,
		"last_name":        lastName,
		"email":            userEmail,
		"password":         string(hashedPwd),
		"role_id":          2,
		"verificado":       1, // Social logins are verified by definition
		"token_expires_at": time.Now().UTC().Add(365 * 24 * time.Hour).Format("2006-01-02 15:04:05"),
	}

	insertRes := r.insertFromMap(usersTable, newUserData, false)
	if insertRes == nil || insertRes == false {
		return nil, fmt.Errorf("Fallo al registrar usuario por login social %s", norm)
	}

	var newUserId int
	err = r.databaseExecutor().QueryRow(fmt.Sprintf("SELECT id FROM %s WHERE email = ? LIMIT 1", usersTable), userEmail).Scan(&newUserId)
	if err != nil || newUserId == 0 {
		return nil, fmt.Errorf("Fallo al recuperar ID de usuario registrado socialmente")
	}

	// Link social account
	insertSocial := map[string]interface{}{
		"user_id":          newUserId,
		"provider":         norm,
		"provider_user_id": userInfo.ProviderUserID,
		"avatar":           userInfo.Avatar,
	}
	r.insertFromMap(socialTable, insertSocial, false)

	return r.createAuthLoginResultForUser(newUserId)
}

func (r *Runtime) exchangeCodeAndFetchUser(provider, clientID, clientSecret, code, redirectURI string) (*SocialUserInfo, error) {
	client := &http.Client{Timeout: 15 * time.Second}

	switch provider {
	case "google":
		tokenURL := "https://oauth2.googleapis.com/token"
		vals := url.Values{
			"code":          {code},
			"client_id":     {clientID},
			"client_secret": {clientSecret},
			"redirect_uri":  {redirectURI},
			"grant_type":    {"authorization_code"},
		}
		resp, err := client.PostForm(tokenURL, vals)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var tokenRes map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&tokenRes)
		accessToken, _ := tokenRes["access_token"].(string)
		if accessToken == "" {
			return nil, fmt.Errorf("No access token in response: %v", tokenRes)
		}

		req, _ := http.NewRequest("GET", "https://www.googleapis.com/oauth2/v3/userinfo", nil)
		req.Header.Set("Authorization", "Bearer "+accessToken)
		uResp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer uResp.Body.Close()
		var uData map[string]interface{}
		json.NewDecoder(uResp.Body).Decode(&uData)

		return &SocialUserInfo{
			Provider:       "google",
			ProviderUserID: fmt.Sprintf("%v", uData["sub"]),
			Email:          fmt.Sprintf("%v", uData["email"]),
			Name:           fmt.Sprintf("%v", uData["name"]),
			FirstName:      fmt.Sprintf("%v", uData["given_name"]),
			LastName:       fmt.Sprintf("%v", uData["family_name"]),
			Avatar:         fmt.Sprintf("%v", uData["picture"]),
		}, nil

	case "github":
		tokenURL := "https://github.com/login/oauth/access_token"
		vals := url.Values{
			"code":          {code},
			"client_id":     {clientID},
			"client_secret": {clientSecret},
			"redirect_uri":  {redirectURI},
		}
		req, _ := http.NewRequest("POST", tokenURL, strings.NewReader(vals.Encode()))
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var tokenRes map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&tokenRes)
		accessToken, _ := tokenRes["access_token"].(string)
		if accessToken == "" {
			return nil, fmt.Errorf("No access token in GitHub response: %v", tokenRes)
		}

		uReq, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
		uReq.Header.Set("Authorization", "Bearer "+accessToken)
		uReq.Header.Set("User-Agent", "Joss-Language-Auth")
		uResp, err := client.Do(uReq)
		if err != nil {
			return nil, err
		}
		defer uResp.Body.Close()
		var uData map[string]interface{}
		json.NewDecoder(uResp.Body).Decode(&uData)

		ghId := fmt.Sprintf("%v", uData["id"])
		ghEmail, _ := uData["email"].(string)
		if ghEmail == "" || ghEmail == "<nil>" {
			// Fetch user emails endpoint
			eReq, _ := http.NewRequest("GET", "https://api.github.com/user/emails", nil)
			eReq.Header.Set("Authorization", "Bearer "+accessToken)
			eReq.Header.Set("User-Agent", "Joss-Language-Auth")
			eResp, err := client.Do(eReq)
			if err == nil {
				defer eResp.Body.Close()
				var emails []map[string]interface{}
				if json.NewDecoder(eResp.Body).Decode(&emails) == nil {
					for _, em := range emails {
						if primary, _ := em["primary"].(bool); primary {
							ghEmail, _ = em["email"].(string)
							break
						}
					}
				}
			}
		}

		name, _ := uData["name"].(string)
		if name == "" {
			name, _ = uData["login"].(string)
		}

		return &SocialUserInfo{
			Provider:       "github",
			ProviderUserID: ghId,
			Email:          ghEmail,
			Name:           name,
			Avatar:         fmt.Sprintf("%v", uData["avatar_url"]),
		}, nil

	case "microsoft":
		tokenURL := "https://login.microsoftonline.com/common/oauth2/v2.0/token"
		vals := url.Values{
			"code":          {code},
			"client_id":     {clientID},
			"client_secret": {clientSecret},
			"redirect_uri":  {redirectURI},
			"grant_type":    {"authorization_code"},
		}
		resp, err := client.PostForm(tokenURL, vals)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var tokenRes map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&tokenRes)
		accessToken, _ := tokenRes["access_token"].(string)
		if accessToken == "" {
			return nil, fmt.Errorf("No access token in Microsoft response: %v", tokenRes)
		}

		uReq, _ := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me", nil)
		uReq.Header.Set("Authorization", "Bearer "+accessToken)
		uResp, err := client.Do(uReq)
		if err != nil {
			return nil, err
		}
		defer uResp.Body.Close()
		var uData map[string]interface{}
		json.NewDecoder(uResp.Body).Decode(&uData)

		email, _ := uData["mail"].(string)
		if email == "" {
			email, _ = uData["userPrincipalName"].(string)
		}

		return &SocialUserInfo{
			Provider:       "microsoft",
			ProviderUserID: fmt.Sprintf("%v", uData["id"]),
			Email:          email,
			Name:           fmt.Sprintf("%v", uData["displayName"]),
			FirstName:      fmt.Sprintf("%v", uData["givenName"]),
			LastName:       fmt.Sprintf("%v", uData["surname"]),
		}, nil

	case "facebook":
		tokenURL := "https://graph.facebook.com/v19.0/oauth/access_token"
		vals := url.Values{
			"code":          {code},
			"client_id":     {clientID},
			"client_secret": {clientSecret},
			"redirect_uri":  {redirectURI},
		}
		resp, err := client.Get(fmt.Sprintf("%s?%s", tokenURL, vals.Encode()))
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var tokenRes map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&tokenRes)
		accessToken, _ := tokenRes["access_token"].(string)
		if accessToken == "" {
			return nil, fmt.Errorf("No access token in Facebook response: %v", tokenRes)
		}

		uURL := fmt.Sprintf("https://graph.facebook.com/me?fields=id,name,first_name,last_name,email,picture.type(large)&access_token=%s", url.QueryEscape(accessToken))
		uResp, err := client.Get(uURL)
		if err != nil {
			return nil, err
		}
		defer uResp.Body.Close()
		var uData map[string]interface{}
		json.NewDecoder(uResp.Body).Decode(&uData)

		avatar := ""
		if pic, ok := uData["picture"].(map[string]interface{}); ok {
			if data, ok := pic["data"].(map[string]interface{}); ok {
				avatar, _ = data["url"].(string)
			}
		}

		return &SocialUserInfo{
			Provider:       "facebook",
			ProviderUserID: fmt.Sprintf("%v", uData["id"]),
			Email:          fmt.Sprintf("%v", uData["email"]),
			Name:           fmt.Sprintf("%v", uData["name"]),
			FirstName:      fmt.Sprintf("%v", uData["first_name"]),
			LastName:       fmt.Sprintf("%v", uData["last_name"]),
			Avatar:         avatar,
		}, nil

	case "x", "twitter":
		tokenURL := "https://api.twitter.com/2/oauth2/token"
		vals := url.Values{
			"code":          {code},
			"client_id":     {clientID},
			"redirect_uri":  {redirectURI},
			"grant_type":    {"authorization_code"},
			"code_verifier": {"challenge"},
		}
		req, _ := http.NewRequest("POST", tokenURL, strings.NewReader(vals.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.SetBasicAuth(clientID, clientSecret)
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var tokenRes map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&tokenRes)
		accessToken, _ := tokenRes["access_token"].(string)
		if accessToken == "" {
			return nil, fmt.Errorf("No access token in X response: %v", tokenRes)
		}

		uReq, _ := http.NewRequest("GET", "https://api.twitter.com/2/users/me?user.fields=profile_image_url,name,username", nil)
		uReq.Header.Set("Authorization", "Bearer "+accessToken)
		uResp, err := client.Do(uReq)
		if err != nil {
			return nil, err
		}
		defer uResp.Body.Close()
		var resMap map[string]interface{}
		json.NewDecoder(uResp.Body).Decode(&resMap)
		data, _ := resMap["data"].(map[string]interface{})

		return &SocialUserInfo{
			Provider:       "x",
			ProviderUserID: fmt.Sprintf("%v", data["id"]),
			Name:           fmt.Sprintf("%v", data["name"]),
			Avatar:         fmt.Sprintf("%v", data["profile_image_url"]),
		}, nil

	case "twitch":
		tokenURL := "https://id.twitch.tv/oauth2/token"
		vals := url.Values{
			"client_id":     {clientID},
			"client_secret": {clientSecret},
			"code":          {code},
			"grant_type":    {"authorization_code"},
			"redirect_uri":  {redirectURI},
		}
		resp, err := client.PostForm(tokenURL, vals)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var tokenRes map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&tokenRes)
		accessToken, _ := tokenRes["access_token"].(string)
		if accessToken == "" {
			return nil, fmt.Errorf("No access token in Twitch response: %v", tokenRes)
		}

		uReq, _ := http.NewRequest("GET", "https://api.twitch.tv/helix/users", nil)
		uReq.Header.Set("Client-Id", clientID)
		uReq.Header.Set("Authorization", "Bearer "+accessToken)
		uResp, err := client.Do(uReq)
		if err != nil {
			return nil, err
		}
		defer uResp.Body.Close()
		var twData map[string]interface{}
		json.NewDecoder(uResp.Body).Decode(&twData)
		var userObj map[string]interface{}
		if list, ok := twData["data"].([]interface{}); ok && len(list) > 0 {
			userObj, _ = list[0].(map[string]interface{})
		}

		return &SocialUserInfo{
			Provider:       "twitch",
			ProviderUserID: fmt.Sprintf("%v", userObj["id"]),
			Email:          fmt.Sprintf("%v", userObj["email"]),
			Name:           fmt.Sprintf("%v", userObj["display_name"]),
			Avatar:         fmt.Sprintf("%v", userObj["profile_image_url"]),
		}, nil

	case "yahoo":
		tokenURL := "https://api.login.yahoo.com/oauth2/get_token"
		vals := url.Values{
			"client_id":     {clientID},
			"client_secret": {clientSecret},
			"redirect_uri":  {redirectURI},
			"code":          {code},
			"grant_type":    {"authorization_code"},
		}
		resp, err := client.PostForm(tokenURL, vals)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		var tokenRes map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&tokenRes)
		accessToken, _ := tokenRes["access_token"].(string)
		if accessToken == "" {
			return nil, fmt.Errorf("No access token in Yahoo response: %v", tokenRes)
		}

		uReq, _ := http.NewRequest("GET", "https://api.login.yahoo.com/v1/userinfo", nil)
		uReq.Header.Set("Authorization", "Bearer "+accessToken)
		uResp, err := client.Do(uReq)
		if err != nil {
			return nil, err
		}
		defer uResp.Body.Close()
		var yData map[string]interface{}
		json.NewDecoder(uResp.Body).Decode(&yData)

		return &SocialUserInfo{
			Provider:       "yahoo",
			ProviderUserID: fmt.Sprintf("%v", yData["sub"]),
			Email:          fmt.Sprintf("%v", yData["email"]),
			Name:           fmt.Sprintf("%v", yData["name"]),
			FirstName:      fmt.Sprintf("%v", yData["given_name"]),
			LastName:       fmt.Sprintf("%v", yData["family_name"]),
			Avatar:         fmt.Sprintf("%v", yData["picture"]),
		}, nil

	case "apple":
		// Apple token endpoint exchange
		tokenURL := "https://appleid.apple.com/auth/token"
		vals := url.Values{
			"client_id":     {clientID},
			"client_secret": {clientSecret},
			"code":          {code},
			"grant_type":    {"authorization_code"},
			"redirect_uri":  {redirectURI},
		}
		resp, err := client.PostForm(tokenURL, vals)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		var tokenRes map[string]interface{}
		json.Unmarshal(bodyBytes, &tokenRes)

		idToken, _ := tokenRes["id_token"].(string)
		if idToken == "" {
			return nil, fmt.Errorf("No id_token in Apple response: %s", string(bodyBytes))
		}

		// Decode payload part of JWT
		parts := strings.Split(idToken, ".")
		if len(parts) < 2 {
			return nil, fmt.Errorf("Invalid Apple id_token JWT structure")
		}
		payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			return nil, fmt.Errorf("Failed to decode Apple id_token payload: %w", err)
		}
		var claims map[string]interface{}
		json.Unmarshal(payloadBytes, &claims)

		return &SocialUserInfo{
			Provider:       "apple",
			ProviderUserID: fmt.Sprintf("%v", claims["sub"]),
			Email:          fmt.Sprintf("%v", claims["email"]),
		}, nil
	}

	return nil, fmt.Errorf("Provider %s not supported", provider)
}

func generateRandomState() string {
	b := make([]byte, 16)
	rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// currentSessionUserId returns the user_id of the currently authenticated session, or 0 if guest
func (r *Runtime) currentSessionUserId() int {
	if sessVal, ok := r.Variables["$__session"]; ok {
		if sessInst, ok := sessVal.(*Instance); ok {
			if uid, ok := sessInst.Fields["user_id"]; ok {
				return toInt(uid)
			}
		}
	}
	return 0
}
