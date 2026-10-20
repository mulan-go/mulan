package vault

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"testing/synctest"
	"time"
)

func TestKeepAliveRetriesRenewalWhenReLoginFails(t *testing.T) {
	for _, ttl := range []int{4, 60} {
		t.Run(fmt.Sprint(ttl), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var logins, renewals int
				var expiresAt time.Time
				v := newTestVault(t, defaultMount, func(r *http.Request) (*http.Response, error) {
					switch r.URL.Path {
					case "/v1/auth/approle/login":
						logins++
						if logins > 1 {
							return jsonResponse(http.StatusForbidden, `{"errors":["secret ID expired"]}`), nil
						}
						expiresAt = time.Now().Add(time.Duration(ttl) * time.Second)
						return jsonResponse(http.StatusOK, fmt.Sprintf(`{"auth":{"client_token":"token","lease_duration":%d,"renewable":true}}`, ttl)), nil
					case "/v1/auth/token/renew-self":
						renewals++
						if !time.Now().Before(expiresAt) {
							t.Error("token expired before renewal recovered")
						}
						if renewals == 1 {
							return jsonResponse(http.StatusServiceUnavailable, `{"errors":["temporarily unavailable"]}`), nil
						}
						expiresAt = time.Now().Add(time.Hour)
						return jsonResponse(http.StatusOK, `{"auth":{"client_token":"token","lease_duration":3600,"renewable":true}}`), nil
					default:
						t.Errorf("unexpected request path %q", r.URL.Path)
						return jsonResponse(http.StatusNotFound, `{}`), nil
					}
				})
				v.client.SetMaxRetries(0)
				if err := v.Login(context.Background()); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan struct{})
				go func() {
					defer close(done)
					v.keepTokenAlive(ctx)
				}()
				synctest.Wait()
				time.Sleep(time.Duration(ttl+1) * time.Second)
				synctest.Wait()
				cancel()
				<-done
				if renewals != 2 || logins != 2 {
					t.Errorf("renewals = %d, logins = %d, want two renewals and one failed re-login", renewals, logins)
				}
			})
		})
	}
}

func TestKeepAliveTracksLoginDuringRetry(t *testing.T) {
	for _, ttl := range []int{0, 2} {
		t.Run(fmt.Sprint(ttl), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var logins, renewals int
				var expiresAt time.Time
				v := newTestVault(t, defaultMount, func(r *http.Request) (*http.Response, error) {
					switch r.URL.Path {
					case "/v1/auth/approle/login":
						logins++
						switch logins {
						case 1:
							return jsonResponse(http.StatusOK, `{"auth":{"client_token":"first","lease_duration":60,"renewable":true}}`), nil
						case 2:
							return jsonResponse(http.StatusForbidden, `{"errors":["temporarily unavailable"]}`), nil
						default:
							expiresAt = time.Now().Add(time.Duration(ttl) * time.Second)
							return jsonResponse(http.StatusOK, fmt.Sprintf(`{"auth":{"client_token":"second","lease_duration":%d,"renewable":true}}`, ttl)), nil
						}
					case "/v1/auth/token/renew-self":
						renewals++
						if renewals == 1 {
							return jsonResponse(http.StatusForbidden, `{"errors":["renewal denied"]}`), nil
						}
						if !time.Now().Before(expiresAt) {
							t.Error("replacement token expired during retry wait")
						}
						if got := r.Header.Get("X-Vault-Token"); got != "second" {
							t.Errorf("renewal token = %q, want second", got)
						}
						return jsonResponse(http.StatusOK, `{"auth":{"client_token":"second","lease_duration":3600,"renewable":true}}`), nil
					default:
						t.Errorf("unexpected request path %q", r.URL.Path)
						return jsonResponse(http.StatusNotFound, `{}`), nil
					}
				})
				if err := v.Login(context.Background()); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan struct{})
				go func() {
					defer close(done)
					v.keepTokenAlive(ctx)
				}()
				synctest.Wait()
				time.Sleep(31 * time.Second)
				synctest.Wait()
				if err := v.Login(ctx); err != nil {
					t.Fatal(err)
				}
				synctest.Wait()
				time.Sleep(10 * time.Second)
				synctest.Wait()
				cancel()
				<-done
				wantRenewals := 1
				if ttl > 0 {
					wantRenewals++
				}
				if renewals != wantRenewals || logins != 3 {
					t.Errorf("renewals = %d, logins = %d, want %d renewals and three logins", renewals, logins, wantRenewals)
				}
			})
		})
	}
}

func TestTokenOperationsCancelWhileLoginIsBlocked(t *testing.T) {
	operations := []struct {
		name string
		run  func(*Vault, context.Context) error
	}{
		{name: "login", run: func(v *Vault, ctx context.Context) error { return v.Login(ctx) }},
		{name: "renew", run: func(v *Vault, ctx context.Context) error { return v.renewToken(ctx, nil) }},
		{name: "keepalive", run: func(v *Vault, ctx context.Context) error {
			v.keepTokenAlive(ctx)
			return ctx.Err()
		}},
	}
	for _, operation := range operations {
		for _, deadline := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/deadline=%t", operation.name, deadline), func(t *testing.T) {
				started := make(chan struct{})
				release := make(chan struct{})
				v := newTestVault(t, defaultMount, func(r *http.Request) (*http.Response, error) {
					if err := r.Context().Err(); err != nil {
						return nil, err
					}
					close(started)
					select {
					case <-release:
						return jsonResponse(http.StatusOK, `{"auth":{"client_token":"token","lease_duration":3600,"renewable":true}}`), nil
					case <-r.Context().Done():
						return nil, r.Context().Err()
					}
				})
				firstDone := make(chan error, 1)
				go func() { firstDone <- v.Login(context.Background()) }()
				<-started

				var ctx context.Context
				var cancel context.CancelFunc
				wantErr := context.Canceled
				if deadline {
					ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
					wantErr = context.DeadlineExceeded
				} else {
					ctx, cancel = context.WithCancel(context.Background())
					timer := time.AfterFunc(10*time.Millisecond, cancel)
					defer timer.Stop()
				}
				defer cancel()
				done := make(chan error, 1)
				go func() { done <- operation.run(v, ctx) }()
				returned := false
				select {
				case err := <-done:
					returned = true
					if !errors.Is(err, wantErr) {
						t.Errorf("operation error = %v, want %v", err, wantErr)
					}
				case <-time.After(time.Second):
					t.Error("canceled operation is still blocked behind another request")
				}
				close(release)
				if err := <-firstDone; err != nil {
					t.Errorf("first Login() error = %v", err)
				}
				if !returned {
					<-done
				}
				// Cancellation must leave synchronization usable by later callers.
				ctx, cancelNext := context.WithTimeout(context.Background(), time.Second)
				defer cancelNext()
				if err := v.renewToken(ctx, nil); err != nil {
					t.Errorf("operation after cancellation error = %v", err)
				}
			})
		}
	}
}

func TestKeepAliveUsesGrantedTTL(t *testing.T) {
	for _, startDelay := range []time.Duration{0, 45 * time.Minute} {
		t.Run(startDelay.String(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var logins, renewals int
				var expiresAt time.Time
				v := newTestVault(t, defaultMount, func(r *http.Request) (*http.Response, error) {
					switch r.URL.Path {
					case "/v1/auth/approle/login":
						logins++
						expiresAt = time.Now().Add(time.Hour)
						return jsonResponse(http.StatusOK, `{"auth":{"client_token":"token","lease_duration":3600,"renewable":true}}`), nil
					case "/v1/auth/token/renew-self":
						renewals++
						if !time.Now().Before(expiresAt) {
							t.Error("token was allowed to expire before renewal")
						}
						if got := r.Header.Get("X-Vault-Token"); got != "token" {
							t.Errorf("renewal token = %q, want token", got)
						}
						// The server grants a shorter TTL than the initial login or requested increment.
						expiresAt = time.Now().Add(15 * time.Minute)
						return jsonResponse(http.StatusOK, `{"auth":{"client_token":"token","lease_duration":900,"renewable":true}}`), nil
					default:
						t.Errorf("unexpected request path %q", r.URL.Path)
						return jsonResponse(http.StatusNotFound, `{}`), nil
					}
				})
				if err := v.Login(context.Background()); err != nil {
					t.Fatal(err)
				}
				time.Sleep(startDelay)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan struct{})
				go func() {
					defer close(done)
					v.keepTokenAlive(ctx)
				}()
				synctest.Wait()
				time.Sleep(75*time.Minute - startDelay)
				synctest.Wait()
				if renewals < 2 {
					t.Errorf("renewals = %d, want at least two before the granted TTLs expire", renewals)
				}
				if logins != 1 {
					t.Errorf("logins = %d, want one while renewal is successful", logins)
				}
				cancel()
				<-done
			})
		})
	}
}

func TestKeepAliveReauthenticatesNonRenewableToken(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var logins, renewals int
		var expiresAt time.Time
		v := newTestVault(t, defaultMount, func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/v1/auth/approle/login":
				if logins > 0 && !time.Now().Before(expiresAt) {
					t.Error("non-renewable token expired before re-login")
				}
				logins++
				expiresAt = time.Now().Add(time.Hour)
				return jsonResponse(http.StatusOK, fmt.Sprintf(`{"auth":{"client_token":"token-%d","lease_duration":3600,"renewable":false}}`, logins)), nil
			case "/v1/auth/token/renew-self":
				renewals++
				return jsonResponse(http.StatusBadRequest, `{"errors":["token is not renewable"]}`), nil
			default:
				t.Errorf("unexpected request path %q", r.URL.Path)
				return jsonResponse(http.StatusNotFound, `{}`), nil
			}
		})
		if err := v.Login(context.Background()); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			v.keepTokenAlive(ctx)
		}()
		synctest.Wait()
		time.Sleep(75 * time.Minute)
		synctest.Wait()
		if logins < 2 || renewals != 0 {
			t.Errorf("logins = %d, renewals = %d, want re-login without renewal", logins, renewals)
		}
		if got, want := v.client.Token(), fmt.Sprintf("token-%d", logins); got != want {
			t.Errorf("client token = %q, want %q", got, want)
		}
		cancel()
		<-done
	})
}

func TestKeepAliveTracksSubsequentLogin(t *testing.T) {
	for _, initialTTL := range []int{0, 7200} {
		t.Run(fmt.Sprint(initialTTL), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var logins, renewals int
				var expiresAt time.Time
				v := newTestVault(t, defaultMount, func(r *http.Request) (*http.Response, error) {
					switch r.URL.Path {
					case "/v1/auth/approle/login":
						logins++
						if logins == 1 {
							return jsonResponse(http.StatusOK, fmt.Sprintf(`{"auth":{"client_token":"first","lease_duration":%d,"renewable":true}}`, initialTTL)), nil
						}
						expiresAt = time.Now().Add(time.Minute)
						return jsonResponse(http.StatusOK, `{"auth":{"client_token":"second","lease_duration":60,"renewable":true}}`), nil
					case "/v1/auth/token/renew-self":
						renewals++
						if !time.Now().Before(expiresAt) {
							t.Error("replacement token expired before renewal")
						}
						if got := r.Header.Get("X-Vault-Token"); got != "second" {
							t.Errorf("renewal token = %q, want second", got)
						}
						expiresAt = time.Now().Add(time.Hour)
						return jsonResponse(http.StatusOK, `{"auth":{"client_token":"second","lease_duration":3600,"renewable":true}}`), nil
					default:
						t.Errorf("unexpected request path %q", r.URL.Path)
						return jsonResponse(http.StatusNotFound, `{}`), nil
					}
				})
				if err := v.Login(context.Background()); err != nil {
					t.Fatal(err)
				}
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				done := make(chan struct{})
				go func() {
					defer close(done)
					v.keepTokenAlive(ctx)
				}()
				synctest.Wait()
				time.Sleep(10 * time.Minute)
				if renewals != 0 {
					t.Errorf("renewals = %d, want no early renewal for initial token", renewals)
				}
				if err := v.Login(ctx); err != nil {
					t.Fatal(err)
				}
				synctest.Wait()
				time.Sleep(time.Minute)
				synctest.Wait()
				if renewals != 1 {
					t.Errorf("renewals = %d, want one before the replacement token expires", renewals)
				}
				cancel()
				<-done
			})
		})
	}
}

func TestKeepAliveRetriesFailedReLogin(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var logins, renewals int
		v := newTestVault(t, defaultMount, func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/v1/auth/approle/login":
				logins++
				if logins == 2 {
					return jsonResponse(http.StatusForbidden, `{"errors":["temporarily unavailable"]}`), nil
				}
				return jsonResponse(http.StatusOK, fmt.Sprintf(`{"auth":{"client_token":"token-%d","lease_duration":3600,"renewable":true}}`, logins)), nil
			case "/v1/auth/token/renew-self":
				renewals++
				return jsonResponse(http.StatusForbidden, `{"errors":["renewal denied"]}`), nil
			default:
				t.Errorf("unexpected request path %q", r.URL.Path)
				return jsonResponse(http.StatusNotFound, `{}`), nil
			}
		})
		if err := v.Login(context.Background()); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			v.keepTokenAlive(ctx)
		}()
		synctest.Wait()
		time.Sleep(time.Hour - time.Second)
		synctest.Wait()
		if renewals == 0 || logins < 3 {
			t.Errorf("renewals = %d, logins = %d, want renewal failure followed by retried login", renewals, logins)
		}
		if got, want := v.client.Token(), fmt.Sprintf("token-%d", logins); got != want {
			t.Errorf("client token = %q, want %q", got, want)
		}
		cancel()
		<-done
	})
}

func TestKeepAliveCancelsInFlightRenewal(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var logins int
		started := make(chan struct{})
		v := newTestVault(t, defaultMount, func(r *http.Request) (*http.Response, error) {
			switch r.URL.Path {
			case "/v1/auth/approle/login":
				logins++
				return jsonResponse(http.StatusOK, `{"auth":{"client_token":"token","lease_duration":3600,"renewable":true}}`), nil
			case "/v1/auth/token/renew-self":
				close(started)
				<-r.Context().Done()
				return nil, r.Context().Err()
			default:
				t.Errorf("unexpected request path %q", r.URL.Path)
				return jsonResponse(http.StatusNotFound, `{}`), nil
			}
		})
		if err := v.Login(context.Background()); err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan struct{})
		go func() {
			defer close(done)
			v.keepTokenAlive(ctx)
		}()
		go func() {
			select {
			case <-started:
				cancel()
			case <-ctx.Done():
			}
		}()
		synctest.Wait()
		time.Sleep(time.Hour - time.Second)
		synctest.Wait()
		select {
		case <-started:
		default:
			t.Error("renewal did not start before token expiry")
		}
		cancel()
		<-done
		if logins != 1 {
			t.Errorf("logins = %d, want no re-login after cancellation", logins)
		}
	})
}
