// Copyright 2021 Trim21 <trim21.me@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.
// See the GNU General Public License for more details.
package hash

import (
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"io"

	"github.com/pkg/errors"
)

func Sha1SumReader(r io.Reader) (string, error) {
	h := sha1.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", errors.Wrap(err, "can't hash content")
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func Sha256SumReaderBytes(r io.Reader) ([]byte, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return nil, errors.Wrap(err, "can't hash content")
	}

	return h.Sum(nil), nil
}

func Sha256SumReader(r io.Reader) (string, error) {
	sum, err := Sha256SumReaderBytes(r)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(sum), nil
}

func Sha1SumBytes(b []byte) []byte {
	h := sha1.New()
	_, _ = h.Write(b)
	sum := h.Sum(nil)

	return sum
}

func Sha1Sum(b []byte) string {
	return hex.EncodeToString(Sha1SumBytes(b))
}

func Sha256SumHex(b []byte) string {
	h := sha256.New()
	_, _ = h.Write(b)
	sum := h.Sum(nil)

	return hex.EncodeToString(sum)
}
