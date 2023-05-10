package util

import (
	"fmt"
	"time"

	"github.com/dgrijalva/jwt-go"
)

func GetToken(sk string, st string, d time.Duration) (string, error) {
	claims := jwt.StandardClaims{
		Subject:   st,
		ExpiresAt: time.Now().Add(d).Unix(),
		//NotBefore: time.Now().Unix(),
		IssuedAt: time.Now().Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(sk))
}

func DecodeToken(sk string, token string) (string, error) {
	var sc jwt.StandardClaims
	t, err := jwt.ParseWithClaims(token, &sc, func(*jwt.Token) (interface {
	}, error) {
		return []byte(sk), nil
	})
	if err != nil {
		return "", fmt.Errorf("invalid token:%s error:%+v", token, err)
	}
	//t.Claims.Valid()
	_, ok := t.Claims.(*jwt.StandardClaims)
	if !ok {
		return "", fmt.Errorf("invalid token:%s not StandardClaims", token)
	}

	return sc.Subject, nil

}

func DecodeTokenWithExpire(sk string, token string) (string, int64, error) {
	var sc jwt.StandardClaims
	t, err := jwt.ParseWithClaims(token, &sc, func(*jwt.Token) (interface {
	}, error) {
		return []byte(sk), nil
	})
	if err != nil {
		return "", 0, fmt.Errorf("invalid token:%s error:%+v", token, err)
	}
	//t.Claims.Valid()
	v, ok := t.Claims.(*jwt.StandardClaims)
	if !ok {
		return "", 0, fmt.Errorf("invalid token:%s not StandardClaims", token)
	}

	return sc.Subject, v.ExpiresAt, nil

}
