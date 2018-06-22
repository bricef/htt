package utils

import "log"

func DieOnError(message string, err error) {
	if err != nil {
		log.Fatal(message, err)
	}
}
