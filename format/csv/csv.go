package csv

import (
	"bytes"
	"encoding/csv"
)

type CSVble interface {
	ToCSV() ([]string, error)
	FromCSV([]string) error
}

func Marshal(res CSVble) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	ts, err := res.ToCSV()
	if err != nil {
		return "", err
	}
	if err := w.Write(ts); err != nil {
		return "", err
	}
	w.Flush()
	return buf.String(), nil
}

func MarshalList(resources []CSVble) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)

	for _, res := range resources {
		ts, err := res.ToCSV()
		if err != nil {
			return "", err
		}
		if err := w.Write(ts); err != nil {
			return "", err
		}
	}

	w.Flush()
	return buf.String(), nil
}
