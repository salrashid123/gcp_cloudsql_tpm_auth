package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"cloud.google.com/go/cloudsqlconn"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	sal "github.com/salrashid123/oauth2/tpm"
)

var (
	tpmPath = flag.String("tpm-path", "/dev/tpm0", "Path to the TPM device (character device or a Unix socket).")

	user                   = flag.String("user", "", "iam user pg-svc-account@$PROJECT_ID.iam")
	serviceAccountEmail    = flag.String("serviceAccountEmail", "", "service-account-email")
	instanceConnectionName = flag.String("instanceConnectionName", "", "Connection Name (eg PROJECT_ID:us-central1:postgres-1)")

	dbName = "postgres"

	persistentHandle = flag.Uint("persistentHandle", 0x81010002, "Handle value")
)

func main() {

	flag.Parse()
	ctx := context.Background()

	ts, err := sal.TpmTokenSource(&sal.TpmTokenConfig{
		TPMPath:       *tpmPath,
		KeyHandle:     uint32(*persistentHandle),
		Email:         *serviceAccountEmail,
		UseOauthToken: true,
	})
	if err != nil {
		log.Printf("error creating tpmTokensource: %v", err)
		os.Exit(1)
	}
	d, err := cloudsqlconn.NewDialer(ctx, cloudsqlconn.WithIAMAuthNTokenSources(ts, ts), cloudsqlconn.WithIAMAuthN())
	if err != nil {
		log.Printf("cloudsqlconn.NewDialer: %v", err)
		os.Exit(1)
	}

	var opts []cloudsqlconn.DialOption

	dsn := fmt.Sprintf("user=%s database=%s sslmode=require", *user, dbName)
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		log.Printf("cloudsqlconn.NewDialer: %v", err)
		os.Exit(1)
	}

	config.DialFunc = func(ctx context.Context, network, instance string) (net.Conn, error) {
		return d.Dial(ctx, *instanceConnectionName, opts...)
	}
	dbURI := stdlib.RegisterConnConfig(config)
	dbPool, err := sql.Open("pgx", dbURI)
	if err != nil {
		log.Printf("cloudsqlconn.NewDialer: %v", err)
		os.Exit(1)
	}

	err = dbPool.Ping()
	if err != nil {
		log.Printf("cloudsqlconn.ping: %v", err)
		os.Exit(1)
	}
	log.Println("Done")

}
