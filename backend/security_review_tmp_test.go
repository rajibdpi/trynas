package main
import("bytes";"net/http";"net/http/httptest";"testing";"time")
func TestSecurityReviewExpiredOffsetSession(t *testing.T){
 a,_,user:=setup(t)
 expired:=time.Now().UTC().Add(-time.Hour).In(time.FixedZone("Dhaka",6*3600))
 if _,err:=a.db.Exec("INSERT INTO sessions(token,user_id,expires_at) VALUES(?,?,?)","audit-expired",user.ID,expired);err!=nil{t.Fatal(err)}
 r:=httptest.NewRequest("GET","/api/me",nil);r.AddCookie(&http.Cookie{Name:"trynas_session",Value:"audit-expired"})
 w:=httptest.NewRecorder();a.auth(http.HandlerFunc(a.me)).ServeHTTP(w,r)
 if w.Code==200 {t.Log("CONFIRMED: a session expired one hour ago in +0600 was accepted") }else{t.Logf("expired session rejected: %d",w.Code)}
}
func TestSecurityReviewCrossOriginLogin(t *testing.T){
 a,_,_:=setup(t)
 r:=httptest.NewRequest("POST","/api/auth/login",bytes.NewBufferString(`{"Username":"test-admin","Password":"test-password","padding":"="}`))
 r.Header.Set("Origin","https://untrusted.example");r.Header.Set("Content-Type","text/plain")
 w:=httptest.NewRecorder();a.cors(http.HandlerFunc(a.login)).ServeHTTP(w,r)
 if w.Code==200 {t.Log("CONFIRMED: cross-origin text/plain login accepted and issued a session")}else{t.Logf("cross-origin login rejected: %d",w.Code)}
}
