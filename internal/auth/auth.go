package auth
 
import (
	"errors"
	"os"
	"time"
 
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)
 
type Claims struct {
	UserID uuid.UUID `json:"user_id"`
	OrgID  uuid.UUID `json:"org_id"`
	Role   string    `json:"role"`
	jwt.RegisteredClaims
}
 
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}


 
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func GenerateToken(userID, orgID uuid.UUID, role string)(string, error){

	secret:=os.Getenv("JWT_SECRET")
	if secret==""{
		return "",errors.New("JWT_SECRET not set")
	}

	claims:=Claims{
		UserID: userID,
		OrgID: orgID,
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24*time.Hour)),
			IssuedAt: jwt.NewNumericDate(time.Now()),
		},
	}

	token:=jwt.NewWithClaims(jwt.SigningMethodHS256,claims)

	return token.SignedString([]byte(secret))

}
 

func ParseToken(tokenString string)(*Claims, error){
	secret:=os.Getenv("JWT_SECRET")
	claims:=&Claims{}

	token,err:=jwt.ParseWithClaims(tokenString,claims, func(t *jwt.Token) ( interface{}, error){
		return []byte(secret),nil
	})
	if err!=nil || !token.Valid{
		return nil,errors.New("invalid token")
	}
	return claims, nil
}
