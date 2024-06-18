package syncer

import "fmt"

type combinedErrors []error

func (errs combinedErrors) Error() string {
	err := "Collected errors:\n"
	for i, e := range errs {
		err += fmt.Sprintf("\tError %d: %s\n", i, e.Error())
	}

	return err
}
