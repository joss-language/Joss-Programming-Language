package core

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"time"
)

// SmtpClient Native Implementation in Joss Core
func (r *Runtime) executeSmtpClientMethod(instance *Instance, method string, args []interface{}) interface{} {
	if instance == nil {
		instance = &Instance{Fields: make(map[string]interface{})}
	} else if instance.Fields == nil {
		instance.Fields = make(map[string]interface{})
	}

	switch method {
	case "auth":
		if len(args) >= 2 {
			instance.Fields["user"] = fmt.Sprintf("%v", args[0])
			instance.Fields["pass"] = fmt.Sprintf("%v", args[1])
		}
		return instance

	case "secure":
		if len(args) >= 1 {
			if b, ok := args[0].(bool); ok {
				instance.Fields["secure"] = b
			}
		}
		return instance

	case "timeout":
		if len(args) >= 1 {
			if sec, ok := args[0].(int64); ok {
				instance.Fields["timeout"] = int(sec)
			} else if sec, ok := args[0].(int); ok {
				instance.Fields["timeout"] = sec
			} else if sec, ok := args[0].(float64); ok {
				instance.Fields["timeout"] = int(sec)
			}
		}
		return instance

	case "lastError":
		if err, ok := instance.Fields["last_error"].(string); ok {
			return err
		}
		return ""

	case "send":
		if len(args) < 3 {
			instance.Fields["last_error"] = "SmtpClient.send requiere ($to, $subject, $body)"
			return false
		}

		to := fmt.Sprintf("%v", args[0])
		subject := fmt.Sprintf("%v", args[1])
		body := fmt.Sprintf("%v", args[2])

		return r.sendSmtpClientMail(instance, to, subject, body)
	}

	return nil
}

func (r *Runtime) sendSmtpClientMail(instance *Instance, to, subject, body string) bool {
	if err := r.RequireCapability("network"); err != nil {
		panic(err)
	}
	host := r.Env["MAIL_HOST"]
	port := r.Env["MAIL_PORT"]
	user := r.Env["MAIL_USERNAME"]
	pass := r.Env["MAIL_PASSWORD"]
	fromAddr := r.Env["MAIL_FROM_ADDRESS"]
	fromName := r.Env["MAIL_FROM_NAME"]

	if u, ok := instance.Fields["user"].(string); ok && u != "" {
		user = u
	}
	if p, ok := instance.Fields["pass"].(string); ok && p != "" {
		pass = p
	}
	if from, ok := instance.Fields["from"].(string); ok && from != "" {
		fromAddr = from
	}
	if name, ok := instance.Fields["from_name"].(string); ok && name != "" {
		fromName = name
	}

	if host == "" {
		host = "smtp.example.com"
	}
	if port == "" {
		port = "587"
	}
	if fromAddr == "" {
		if user != "" {
			fromAddr = user
		} else {
			fromAddr = "noreply@example.com"
		}
	}
	if fromName == "" {
		fromName = "Joss App"
	}

	timeoutSec := 30
	if t, ok := instance.Fields["timeout"].(int); ok && t > 0 {
		timeoutSec = t
	}

	addr := net.JoinHostPort(host, port)

	var client *smtp.Client
	var err error

	if port == "465" {
		tlsConfig := &tls.Config{
			ServerName:         host,
			InsecureSkipVerify: false,
		}
		conn, dialErr := tls.DialWithDialer(&net.Dialer{Timeout: time.Duration(timeoutSec) * time.Second}, "tcp", addr, tlsConfig)
		if dialErr != nil {
			instance.Fields["last_error"] = fmt.Sprintf("Error conectando TLS a %s: %v", addr, dialErr)
			return false
		}
		client, err = smtp.NewClient(conn, host)
		if err != nil {
			instance.Fields["last_error"] = fmt.Sprintf("Error inicializando cliente SMTP: %v", err)
			return false
		}
	} else {
		conn, dialErr := net.DialTimeout("tcp", addr, time.Duration(timeoutSec)*time.Second)
		if dialErr != nil {
			instance.Fields["last_error"] = fmt.Sprintf("Error conectando a %s: %v", addr, dialErr)
			return false
		}
		client, err = smtp.NewClient(conn, host)
		if err != nil {
			instance.Fields["last_error"] = fmt.Sprintf("Error inicializando cliente SMTP: %v", err)
			return false
		}

		if port == "587" {
			if ok, _ := client.Extension("STARTTLS"); ok {
				if tlsErr := client.StartTLS(&tls.Config{ServerName: host}); tlsErr != nil {
					instance.Fields["last_error"] = fmt.Sprintf("Error en STARTTLS: %v", tlsErr)
					return false
				}
			}
		}
	}
	defer client.Quit()

	if user != "" && pass != "" {
		auth := smtp.PlainAuth("", user, pass, host)
		if authErr := client.Auth(auth); authErr != nil {
			instance.Fields["last_error"] = fmt.Sprintf("Autenticación SMTP fallida: %v", authErr)
			return false
		}
	}

	if mailErr := client.Mail(fromAddr); mailErr != nil {
		instance.Fields["last_error"] = fmt.Sprintf("Error en MAIL FROM (%s): %v", fromAddr, mailErr)
		return false
	}
	if rcptErr := client.Rcpt(to); rcptErr != nil {
		instance.Fields["last_error"] = fmt.Sprintf("Error en RCPT TO (%s): %v", to, rcptErr)
		return false
	}

	w, dataErr := client.Data()
	if dataErr != nil {
		instance.Fields["last_error"] = fmt.Sprintf("Error iniciando DATA: %v", dataErr)
		return false
	}

	msg := fmt.Sprintf("From: %s <%s>\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n%s",
		fromName, fromAddr, to, subject, body)

	if _, writeErr := w.Write([]byte(msg)); writeErr != nil {
		instance.Fields["last_error"] = fmt.Sprintf("Error escribiendo cuerpo de correo: %v", writeErr)
		return false
	}
	if closeErr := w.Close(); closeErr != nil {
		instance.Fields["last_error"] = fmt.Sprintf("Error finalizando envío: %v", closeErr)
		return false
	}

	instance.Fields["last_error"] = ""
	return true
}
