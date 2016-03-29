package mail

import (
	"fmt"
	"io"
	"net/url"

	mailgun "github.com/mailgun/mailgun-go"
)

func newMailgun(addr string) (mailgun.Mailgun, error) {
	u, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	if u.User == nil {
		return nil, fmt.Errorf("mailgun: user must be defined")
	}
	user := u.User.Username()
	pass, _ := u.User.Password()

	return mailgun.NewMailgun(u.Host, user, pass), nil
}

func SendFile(addr, from, subj, text, name string, file io.ReadCloser, to ...string) error {
	mgn, err := newMailgun(addr)
	if err != nil {
		return err
	}

	msg := mgn.NewMessage(from, subj, text, to...)
	if file != nil {
		msg.AddReaderAttachment(name, file)
	}

	if _, _, err = mgn.Send(msg); err != nil {
		return err
	}

	return nil
}

func Send(addr, from, subj, text string, to ...string) error {
	return SendFile(addr, from, subj, text, "", nil, to...)
}
