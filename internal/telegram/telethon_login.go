package telegram

import (
	"context"
	"fmt"

	"github.com/gotd/td/session"
	gotdtelegram "github.com/gotd/td/telegram"
)

func (g *GotdClient) LoginWithTelethonSession(ctx context.Context, sessionString string, sessionPath string) (Profile, error) {
	data, err := session.TelethonSession(sessionString)
	if err != nil {
		return Profile{}, fmt.Errorf("invalid telethon session: %w", err)
	}

	loader := &session.Loader{Storage: &session.FileStorage{Path: sessionPath}}
	if err := loader.Save(ctx, data); err != nil {
		return Profile{}, fmt.Errorf("save session: %w", err)
	}

	var profile Profile
	err = g.withClient(ctx, sessionPath, func(ctx context.Context, client *gotdtelegram.Client) error {
		user, err := client.Self(ctx)
		if err != nil {
			return err
		}
		profile = profileFromUser(user)
		return nil
	})
	if err != nil {
		return Profile{}, fmt.Errorf("fetch profile: %w", err)
	}
	return profile, nil
}
