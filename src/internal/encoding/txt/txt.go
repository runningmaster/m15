package txt

import (
	"io"

	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/transform"
)

// Win1251ToUTF8 decodes text from Win1251 to UTF8
func Win1251ToUTF8(r io.Reader) io.Reader {
	return transform.NewReader(r, charmap.Windows1251.NewDecoder())
}

/*
   toWin1251 := func(s string) string {
           b := new(bytes.Buffer)
           w := transform.NewWriter(b, charmap.Windows1251.NewEncoder())
           _, _ = w.Write([]byte(s))
           return b.String()
   }

*/
