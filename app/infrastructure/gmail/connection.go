package gmail

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
)

// ConnectionResult is the outcome of validating a Gmail address + App
// Password pair against Gmail's SMTP server, without sending anything —
// mirrors outlook.ConnectionResult so both providers report status the
// same shape to app.go.
type ConnectionResult struct {
	Available bool
	Email     string
	Detail    string
}

// TestConnection dials Gmail's SMTP server, negotiates STARTTLS, and
// authenticates with the given App Password. It sends no mail — MAIL/RCPT/
// DATA are never issued — so it is safe to call repeatedly from the UI
// while the user is setting up the connection.
func TestConnection(email, appPassword string) ConnectionResult {
	client, err := smtp.Dial(smtpAddr)
	if err != nil {
		return ConnectionResult{
			Available: false,
			Detail:    fmt.Sprintf("No se pudo conectar a Gmail (smtp.gmail.com:587): %v", err),
		}
	}
	defer client.Close()

	if err := client.StartTLS(&tls.Config{ServerName: smtpHost}); err != nil {
		return ConnectionResult{
			Available: false,
			Detail:    fmt.Sprintf("No se pudo iniciar TLS con Gmail: %v", err),
		}
	}

	auth := smtp.PlainAuth("", email, appPassword, smtpHost)
	if err := client.Auth(auth); err != nil {
		return ConnectionResult{
			Available: false,
			Detail:    "Gmail rechazó las credenciales. Verifica el correo y la contraseña de aplicación (16 caracteres, generada en myaccount.google.com/apppasswords).",
		}
	}

	return ConnectionResult{
		Available: true,
		Email:     email,
		Detail:    "Conexión con Gmail establecida.",
	}
}
