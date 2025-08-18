package llm

import "errors"

// small helper to keep return type consistent
func nilStringErr(msg string) (string, error) { return "", errors.New(msg) }
