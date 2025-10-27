package ktpn

import (
	"time"
)

func Retry[T any](n int, f func() (T, error)) (result T, err error) {
	attempt := 0
	for attempt < n {
		result, err = f()
		if err != nil {
			attempt++
			time.Sleep(time.Duration(attempt) * time.Second)
		} else {
			break
		}
	}
	return result, err
}
