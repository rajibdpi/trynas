package core

type AppSettings struct {
	RegistrationEnabled bool   `json:"registrationEnabled"`
	AppName             string `json:"appName"`
	LogoURL             string `json:"logoUrl"`
	FooterText          string `json:"footerText"`
	FileView            string `json:"fileView"`
	ShareExpiry         string `json:"shareExpiry"`
}
