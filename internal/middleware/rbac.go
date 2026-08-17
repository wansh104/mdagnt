

package middleware 

import (
	"context"
	"net/http"
  "strings"
  
	
	
	
	"medagent/internal/auth"

)

type contextKey string
const ClaimsKey contextKey = "claims"


func RequireAuth(next http.Handler) http.Handler{
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request){
		
		header:=r.Header.Get("Authorization")
		if !strings.HasPrefix(header, "Bearer "){
			http.Error(w,"missing bearer token",http.StatusUnauthorized)
			return
		}

		token:=strings.TrimPrefix(header, "Bearer ")

		claims, err:=auth.ParseToken(token)
		if err!=nil{
			http.Error(w,"invalid token", http.StatusUnauthorized)
			return
		}
		ctx:= context.WithValue(r.Context(), ClaimsKey, claims)
		next.ServeHTTP(w,r.WithContext(ctx))

	})
}

func RequireRole(roles ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			claims, ok := r.Context().Value(ClaimsKey).(*auth.Claims)
			if !ok {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			for _, role := range roles {
				if claims.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			http.Error(w, "forbidden: insufficient role", http.StatusForbidden)
		})
	}
}