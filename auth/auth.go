package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/rajibdpi/trynas/core"
	"golang.org/x/crypto/bcrypt"
)

func (a *Module) Cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !core.CheckBrowserMutation(w, r) {
			return
		}
		w.Header().Add("Vary", "Origin")
		o := core.Env("CORS_ORIGIN", "http://localhost:3000")
		if r.Header.Get("Origin") == o {
			w.Header().Set("Access-Control-Allow-Origin", o)
		}
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
		if r.Method == "OPTIONS" {
			w.WriteHeader(204)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *Module) Login(w http.ResponseWriter, r *http.Request) {
	var x struct{ Username, Password string }
	if json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&x) != nil {
		core.Bad(w, "bad request", 400)
		return
	}
	if !a.AllowAttempt(w, r, "login:"+strings.ToLower(x.Username)) {
		return
	}
	var u core.User
	var h string
	err := a.DB.QueryRow("SELECT id,username,role,password_hash FROM users WHERE username=?", x.Username).Scan(&u.ID, &u.Username, &u.Role, &h)
	if err != nil || bcrypt.CompareHashAndPassword([]byte(h), []byte(x.Password)) != nil {
		core.Bad(w, "invalid credentials", 401)
		return
	}
	t := core.Token()
	exp := time.Now().UTC().Add(7 * 24 * time.Hour)
	if _, err = a.DB.Exec("INSERT INTO sessions(token,user_id,expires_at) VALUES(?,?,?)", t, u.ID, exp.Format(time.RFC3339Nano)); err != nil {
		core.Bad(w, "could not create session", 500)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: "trynas_session", Value: t, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode, Expires: exp, Secure: core.SecureCookie(r)})
	a.Log(u.ID, "login", "")
	core.JSON(w, a.Profile(u))
}

func (a *Module) Logout(w http.ResponseWriter, r *http.Request) {
	if c, e := r.Cookie("trynas_session"); e == nil {
		_, _ = a.DB.Exec("DELETE FROM sessions WHERE token=?", c.Value)
	}
	http.SetCookie(w, &http.Cookie{Name: "trynas_session", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode, Secure: core.SecureCookie(r)})
	core.JSON(w, map[string]bool{"ok": true})
}

func (a *Module) Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		c, e := r.Cookie("trynas_session")
		if e != nil {
			core.Bad(w, "unauthorized", 401)
			return
		}
		var u core.User
		e = a.DB.QueryRow(`SELECT u.id,u.username,u.role FROM sessions s JOIN users u ON u.id=s.user_id WHERE s.token=? AND julianday(s.expires_at)>julianday('now')`, c.Value).Scan(&u.ID, &u.Username, &u.Role)
		if e != nil {
			core.Bad(w, "unauthorized", 401)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), core.UserKey, u)))
	})
}

func (a *Module) Me(w http.ResponseWriter, r *http.Request) {
	core.JSON(w, a.Profile(r.Context().Value(core.UserKey).(core.User)))
}
