package core

type AppSettings struct {
	RegistrationEnabled bool   `json:"registrationEnabled"`
	AppName             string `json:"appName"`
	FileView            string `json:"fileView"`
	ShareExpiry         string `json:"shareExpiry"`
}
