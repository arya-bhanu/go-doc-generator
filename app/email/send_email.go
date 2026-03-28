package email

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"strconv"

	"gopkg.in/gomail.v2"
)

type Attachment struct {
	Filename string
	Data     []byte
}

func SendEmail(emailSubject string, files []Attachment, emailTo string) {
	if len(files) == 0 {
		slog.Warn("No attachments provided, skipping email send")
		return
	}

	smtpHost := os.Getenv("SMTP_HOST")
	smtpPortStr := os.Getenv("SMTP_PORT")
	smtpUser := os.Getenv("SMTP_USER")
	smtpPassword := os.Getenv("SMTP_PASSWORD")
	emailFrom := os.Getenv("EMAIL_FROM")

	smtpPort, err := strconv.Atoi(smtpPortStr)
	if err != nil {
		slog.Error("Invalid SMTP_PORT value", "error", err)
		return
	}

	m := gomail.NewMessage()
	m.SetHeader("From", emailFrom)
	m.SetHeader("To", emailTo)
	m.SetHeader("Subject", emailSubject)
	m.SetBody("text/plain", "Please find the attached document(s).")

	for _, f := range files {
		file := f
		m.Attach(file.Filename, gomail.SetCopyFunc(func(w io.Writer) error {
			_, err := bytes.NewReader(file.Data).WriteTo(w)
			return err
		}))
		slog.Info("Attaching file", "filename", file.Filename, "size_bytes", len(file.Data))
	}

	d := gomail.NewDialer(smtpHost, smtpPort, smtpUser, smtpPassword)
	if err := d.DialAndSend(m); err != nil {
		slog.Error("Failed to send email", "error", err)
		return
	}

	slog.Info("Email sent successfully", "attachments", len(files))
}
