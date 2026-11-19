package main

import (
	"flag"
	"log"

	"github.com/gravypower/dd"
	ddapi "github.com/gravypower/dd/api"
	"github.com/gravypower/dd/helper"
)

var (
	flagCredentialsPath = flag.String("credentials", "dd-credentials.json", "path to credentials file")
	flagShareCode       = flag.String("code", "", "share code")
	flagPassword        = flag.String("password", "", "password")
	flagPhoneInfo       = flag.String("phone", "API", "phone info to report")
)

func main() {
	flag.Parse()

	if *flagShareCode == "" || *flagPassword == "" {
		log.Fatalf("must specify -code and -password")
	}

	req := ddapi.RegisterRequest{
		RemoteRegistrationCode: *flagShareCode,
		UserPassword:           *flagPassword,
		PhoneName:              *flagPhoneInfo,
		PhoneModel:             *flagPhoneInfo,
	}
	out := ddapi.RegisterResponse{}

	conn := dd.Conn{}
	err := conn.SimpleRequest(dd.SimpleRequest{
		Path:   "/app/remoteregister",
		Target: dd.RemoteTarget,
		Input:  req,
		Output: &out,
	})
	if err != nil {
		// The request and response hold the password and secrets, so only the error is logged.
		log.Fatalf("can't remoteregister: %v", err)
	}

	out.UserPassword = *flagPassword

	err = helper.SaveCreds(*flagCredentialsPath, &out)
	if err != nil {
		log.Fatalf("can't encode and save response: %v", err)
	}

	log.Printf("Ok! Saved encrypted credentials at: %v", *flagCredentialsPath)
}
