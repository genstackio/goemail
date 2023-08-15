package goemail

type Email struct {
	Type        string        `json:"type"`
	Cc          []string      `json:"cc"`
	Bcc         []string      `json:"bcc"`
	To          []string      `json:"to"`
	Body        string        `json:"body"`
	BodyText    string        `json:"bodyText"`
	Subject     string        `json:"subject"`
	Attachments []*Attachment `json:"attachments"`
	Vars        Vars          `json:"vars"`
	From        string        `json:"from,omitempty"`
	FromArn     string        `json:"fromArn,omitempty"`
	FromEmail   string        `json:"fromEmail,omitempty"`
}

type Attachment struct {
	Name      string `json:"name"`
	Source    string `json:"source"`
	Important bool   `json:"important"`
	Package   string `json:"package"`
	Content   []byte
	Treated   bool
}

type Vars struct {
	LastName  string `json:"lastName"`
	FirstName string `json:"firstName"`
}
