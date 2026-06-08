package telegram

import (
	"context"
	"fmt"
	"sync"
	"time"

	gotdsession "github.com/gotd/td/session"
	gotdtelegram "github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth/qrlogin"
	"github.com/gotd/td/tg"
)

type gotdQRLoginSession struct {
	mu       sync.Mutex
	cancel   context.CancelFunc
	done     chan error
	qr       qrlogin.QR
	token    QRLoginToken
	profile  Profile
	accepted bool
	complete bool
}

func (g *GotdClient) StartQRLogin(ctx context.Context, sessionPath string) (QRLoginSession, error) {
	if sessionPath == "" {
		return nil, fmt.Errorf("qr login session path is required")
	}
	runCtx, cancel := context.WithCancel(context.Background())
	session := &gotdQRLoginSession{
		cancel: cancel,
		done:   make(chan error, 1),
	}
	dispatcher := tg.NewUpdateDispatcher()
	dispatcher.OnLoginToken(func(ctx context.Context, e tg.Entities, update *tg.UpdateLoginToken) error {
		session.markAccepted()
		return nil
	})
	client := gotdtelegram.NewClient(g.apiID, g.apiHash, gotdtelegram.Options{
		SessionStorage: &gotdsession.FileStorage{Path: sessionPath},
		Logger:         g.logger,
		UpdateHandler:  dispatcher,
	})

	ready := make(chan error, 1)
	var readyOnce sync.Once
	sendReady := func(err error) {
		readyOnce.Do(func() {
			ready <- err
		})
	}
	go func() {
		err := client.Run(runCtx, func(clientCtx context.Context) error {
			qr := client.QR()
			token, err := qr.Export(clientCtx)
			if err == nil {
				session.mu.Lock()
				session.qr = qr
				session.token = qrLoginTokenFromGotd(token)
				session.mu.Unlock()
			}
			sendReady(err)
			if err != nil {
				return err
			}
			<-runCtx.Done()
			return runCtx.Err()
		})
		sendReady(err)
		session.done <- err
	}()

	select {
	case err := <-ready:
		if err != nil {
			cancel()
			return nil, err
		}
		return session, nil
	case <-ctx.Done():
		cancel()
		return nil, ctx.Err()
	}
}

func (s *gotdQRLoginSession) Token() QRLoginToken {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.token
}

func (s *gotdQRLoginSession) Poll(ctx context.Context) (QRLoginPollResult, error) {
	s.mu.Lock()
	if s.complete {
		profile := s.profile
		s.mu.Unlock()
		return QRLoginPollResult{Status: QRLoginStatusOnline, Profile: profile}, nil
	}
	accepted := s.accepted
	token := s.token
	qr := s.qr
	s.mu.Unlock()

	if accepted {
		return s.importAccepted(ctx, qr)
	}
	if !token.ExpiresAt.IsZero() && time.Now().UTC().After(token.ExpiresAt) {
		refreshed, err := qr.Export(ctx)
		if err != nil {
			return QRLoginPollResult{}, err
		}
		token = qrLoginTokenFromGotd(refreshed)
		s.mu.Lock()
		s.token = token
		s.mu.Unlock()
	}
	return QRLoginPollResult{Status: QRLoginStatusPending, Token: token}, nil
}

func (s *gotdQRLoginSession) Cancel(ctx context.Context) error {
	s.cancel()
	select {
	case <-s.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
		return nil
	}
}

func (s *gotdQRLoginSession) markAccepted() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.accepted = true
}

func (s *gotdQRLoginSession) importAccepted(ctx context.Context, qr qrlogin.QR) (QRLoginPollResult, error) {
	auth, err := qr.Import(ctx)
	if err != nil {
		return QRLoginPollResult{}, err
	}
	profile := profileFromAuthorization(auth)
	s.mu.Lock()
	s.profile = profile
	s.complete = true
	s.mu.Unlock()
	s.cancel()
	return QRLoginPollResult{Status: QRLoginStatusOnline, Profile: profile}, nil
}

func qrLoginTokenFromGotd(token qrlogin.Token) QRLoginToken {
	return QRLoginToken{URL: token.URL(), ExpiresAt: token.Expires()}
}
