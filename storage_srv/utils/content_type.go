package utils

import "mime"

func ContentType(filename string) string {
	if value := mime.TypeByExtension(filename); value != "" {
		return value
	}
	return "application/octet-stream"
}
