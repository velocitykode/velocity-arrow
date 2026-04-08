# Velocity Mail

## Drivers

- `postmark` - Postmark transactional email
- `mailgun` - Mailgun email service
- `log` - Logs email to application log (development)

## Configuration

- `MAIL_DRIVER` - postmark, mailgun, or log
- Driver-specific env vars for API keys and domains

## Usage

```go
err := services.Mail.Send(mail.Message{
    To:      []string{"user@example.com"},
    Subject: "Welcome",
    Body:    "Hello!",
})
```

## Rules

- Use the `log` driver in development - never send real emails in dev/test
- Queue emails for async sending in production
- Validate email addresses before sending
