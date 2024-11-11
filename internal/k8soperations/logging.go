package k8soperations

import "log/slog"

type slogWriter struct {
	stream  string
	message string
}

func (sw slogWriter) Write(p []byte) (n int, err error) {
	output := string(p)
	switch sw.stream {
	case "stdout":
		slog.Info(sw.message, "stdout", output)
	case "stderr":
		slog.Info(sw.message, "stderr", output)
	}
	return len(p), nil
}
