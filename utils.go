package goemail

import (
	"archive/zip"
	"bytes"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ses"
	"github.com/go-gomail/gomail"
	"github.com/ohoareau/goaws/s3"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
)

func cleanEmails(emails []string) []string {
	if 0 == len(emails) {
		return emails
	}
	for k, v := range emails {
		emails[k] = cleanEmail(v)
	}
	return emails
}

func cleanEmail(email string) string {
	if 0 == len(email) {
		return email
	}
	email = strings.ReplaceAll(email, "@@", "@")

	return email
}

func createStandardEmail(data *Email) *ses.SendEmailInput {
	var dest, cc, bcc *string
	html := data.Body
	text := data.BodyText
	subject := data.Subject
	data.To = cleanEmails(data.To)
	data.Bcc = cleanEmails(data.Bcc)
	data.Cc = cleanEmails(data.Cc)
	if len(data.Bcc) > 0 {
		bcc = aws.StringSlice(data.Bcc)[len(data.Bcc)-1]
	}
	if len(data.Cc) > 0 {
		cc = aws.StringSlice(data.Cc)[len(data.Cc)-1]
	}
	if len(data.To) > 0 {
		dest = aws.StringSlice(data.To)[len(data.To)-1]
	}
	from, fromArn := buildFrom(data.FromEmail, data.From, data.FromArn)
	input := &ses.SendEmailInput{
		Destination: &ses.Destination{
			BccAddresses: []*string{bcc},
			CcAddresses:  []*string{cc},
			ToAddresses:  []*string{dest},
		},
		Message: &ses.Message{
			Body: &ses.Body{
				Html: &ses.Content{
					Data: aws.String(html),
				},
				Text: &ses.Content{
					Data: aws.String(text),
				},
			},
			Subject: &ses.Content{
				Data: aws.String(subject),
			},
		},
		Source:    aws.String(from),
		SourceArn: aws.String(fromArn),
	}
	return input
}

func buildFrom(email string, name string, arn string) (string, string) {
	if len(email) == 0 {
		email = os.Getenv("EMAIL_IDENTITY_NOREPLY_EMAIL") // default value
	}
	if len(arn) == 0 {
		arn = os.Getenv("EMAIL_IDENTITY_NOREPLY_ARN") // default value
	}

	if "" != name {
		return name + " <" + email + ">", arn
	}
	return email, arn
}

func createEmailWithAttachments(data *Email) (*ses.SendRawEmailInput, error) {
	var destinations *string
	if len(data.To) > 0 {
		destinations = aws.StringSlice(data.To)[len(data.To)-1]
	}
	raw, err := createRawMessage(data)
	if err != nil {
		return nil, err
	}
	from, fromArn := buildFrom(data.FromEmail, data.From, data.FromArn)

	input := &ses.SendRawEmailInput{
		Destinations: []*string{destinations},
		Source:       aws.String(from),
		SourceArn:    aws.String(fromArn),
		RawMessage:   raw,
	}
	return input, nil
}

func createRawMessage(data *Email) (*ses.RawMessage, error) {
	var dest string
	var err error
	if len(data.To) > 0 {
		dest = strings.Join(data.To, "")
	}
	from, _ := buildFrom(data.FromEmail, data.From, data.FromArn)
	html := data.Body
	subject := data.Subject
	msg := gomail.NewMessage()
	msg.SetHeader("From", from)
	msg.SetHeader("To", dest)
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/html", html)
	for i := 0; i < len(data.Attachments); i++ {
		err = getObjectInByte(data.Attachments[i])
		if err != nil {
			return nil, err
		}
	}
	err = prepareRawEmail(msg, data)
	if err != nil {
		return nil, err
	}
	var emailRaw bytes.Buffer
	msg.WriteTo(&emailRaw)
	message := ses.RawMessage{Data: emailRaw.Bytes()}
	return &message, nil
}

func prepareRawEmail(msg *gomail.Message, data *Email) error {
	checkImportantAttachments(data.Attachments)
	for i := 0; i < len(data.Attachments); i++ {
		packName := data.Attachments[i].Package
		treatPackage := getPackageInputs(data.Attachments, packName)
		size := getPackSize(treatPackage, packName)
		organizeAttachments(msg, treatPackage, packName, size)
	}
	return nil
}

func getObjectInByte(att *Attachment) error {
	var err error
	cmp := strings.Split(att.Source, "/")
	if cmp[0] == "s3:" {
		att.Content, err = s3.GetObject(cmp[2], cmp[3]+"/"+cmp[4])
		if err != nil {
			return err
		}
		return nil
	}
	if cmp[0] == "http:" || cmp[0] == "https:" {
		file, err := http.Get(att.Source)
		if err != nil {
			return err
		}

		att.Content, err = ioutil.ReadAll(file.Body)
		if err != nil {
			return err
		}
		return nil
	}
	return nil
}

func getPackageInputs(att []*Attachment, packName string) []Attachment {
	var newAttachment []Attachment
	for i := 0; i < len(att); i++ {
		if att[i].Package != packName || att[i].Treated {
			continue
		}
		newAttachment = append(newAttachment, *att[i])
		att[i].Treated = true
	}
	return newAttachment
}

func checkImportantAttachments(att []*Attachment) {
	MaxImportantSize := 10000000
	for i := 0; i < len(att); i++ {
		if att[i].Important && len(att[i].Content) < MaxImportantSize {
			continue
		}
		att[i].Important = false
	}
}

func getPackSize(att []Attachment, packName string) int {
	var size int = 0
	for i := 0; i < len(att); i++ {
		if att[i].Important {
			continue
		}
		size += len(att[i].Content)
	}
	return size
}

func organizeAttachments(msg *gomail.Message, attachments []Attachment, packName string, size int) error {
	MinZipSize := 15000000
	if size >= MinZipSize {
		err := zipPackage(msg, attachments, packName)
		if err != nil {
			return err
		}
	} else {
		err := addMultipleAttachments(msg, attachments, packName)
		if err != nil {
			return err
		}
	}
	return nil
}

func addMultipleAttachments(msg *gomail.Message, attachments []Attachment, packName string) error {
	for i := 0; i < len(attachments); i++ {
		if attachments[i].Package != packName {
			continue
		}
		addAttachment(msg, attachments[i])
		attachments[i].Treated = true
	}
	return nil
}

func zipPackage(msg *gomail.Message, att []Attachment, packName string) error {
	var useName string
	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)
	for i := 0; i < len(att); i++ {
		if att[i].Important {
			addAttachment(msg, att[i])
			continue
		}
		addToZip(w, att[i])
	}
	err := w.Close()
	if err != nil {
		return err
	}
	if packName == "" {
		useName = "Attachments" + strconv.Itoa(rand.Int()) + ".zip"
	} else {
		useName = packName
	}
	msg.Attach(useName, gomail.SetCopyFunc(func(z io.Writer) error {
		_, err := buf.WriteTo(z)
		return err
	}))
	return nil
}

func addToZip(w *zip.Writer, attachment Attachment) error {
	f, err := w.Create(attachment.Name)
	if err != nil {
		return err
	}
	_, err = f.Write(attachment.Content)
	if err != nil {
		return err
	}
	return nil
}

func addAttachment(msg *gomail.Message, att Attachment) {
	msg.Attach(att.Name, gomail.SetCopyFunc(func(w io.Writer) error {
		_, err := w.Write(att.Content)
		return err
	}))
}
