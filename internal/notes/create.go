package notes

import (
	"fmt"
	"os"
)

// Create writes exclusively; failed writes remove only this invocation's file.
// Parents are created by the caller after all template/editor validation.
func Create(root, rel, contents string) error {
	p, e := Safe(root, rel, true)
	if e != nil {
		return e
	}
	f, e := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0666)
	if e != nil {
		return e
	}
	n, we := f.WriteString(contents)
	ce := f.Close()
	if we != nil || n != len(contents) || ce != nil {
		os.Remove(p)
		if we != nil {
			return we
		}
		if ce != nil {
			return ce
		}
		return fmt.Errorf("incomplete note write")
	}
	return nil
}
