package desktop

import (
	"context"
	"net/url"
)

type Commands struct {
	Quit func()
}

func URLWithAdminToken(baseURL, token string) (string, error) {
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	fragment := url.Values{"token": []string{token}}
	parsed.Fragment = fragment.Encode()
	return parsed.String(), nil
}

func RunNative(ctx context.Context, pageURL string, commands Commands) error {
	return runNative(ctx, pageURL, commands)
}
