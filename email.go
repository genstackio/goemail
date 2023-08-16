package goemail

import (
	"errors"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
)

//goland:noinspection GoUnusedExportedFunction
func SendEmail(email *Email) (string, error) {
	if len(email.Bcc) <= 0 && len(email.Cc) <= 0 && len(email.To) <= 0 {
		return "", errors.New("no recipients specified")
	}
	sess, _ := session.NewSession()
	svc := ses.New(sess)

	var messageId string

	if len(email.Attachments) > 0 {
		input, err := createEmailWithAttachments(email)
		if err != nil {
			return messageId, err
		}
		x, err := svc.SendRawEmail(input)
		messageId = *x.MessageId
		return messageId, err
	}

	input := createStandardEmail(email)
	x, err := svc.SendEmail(input)

	if nil != err {
		return "", err
	}
	messageId = *x.MessageId
	return messageId, err
}
