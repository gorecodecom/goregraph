package watch

import "time"

func retryStateIO(operation func() error) error {
	var err error
	for attempt := 0; attempt < 20; attempt++ {
		err = operation()
		if !stateSharingViolation(err) {
			return err
		}
		if attempt < 19 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	return err
}
