package main

import (
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/google/go-tpm-tools/client"
	"github.com/google/go-tpm/legacy/tpm2"
	"github.com/google/go-tpm/tpmutil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"

	// sal "github.com/salrashid123/signer/tpm"
	// "github.com/google/go-tpm-tools/client"
	// "github.com/google/go-tpm/legacy/tpm2"
	// "github.com/google/go-tpm/tpmutil"
	sal "github.com/salrashid123/signer/tpm"
)

var (
	dbUser = "postgres"
	dbName = "postgres"

	tpmPath    = flag.String("tpm-path", "/dev/tpm0", "Path to the TPM device (character device or a Unix socket).")
	cacert     = flag.String("cacert", "server-ca.pem", "RootCA")
	publicCert = flag.String("publicCert", "client-cert.pem", "Client public cert")
	password   = flag.String("password", "iTi3KsuGtz", "Database root password")

	host = flag.String("host", "", "Database public IP")
	sni  = flag.String("sni", "foo", "Database root password")

	persistentHandle = flag.Uint("persistentHandle", 0x81010003, "Handle value")
)

func main() {

	flag.Parse()

	caCert, err := os.ReadFile(*cacert)
	if err != nil {
		log.Fatal(err)
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(caCert)

	rwc, err := tpm2.OpenTPM(*tpmPath)
	if err != nil {
		log.Fatalf("can't open TPM %q: %v", *tpmPath, err)
	}
	defer func() {
		if err := rwc.Close(); err != nil {
			log.Fatalf("can't close TPM %q: %v", *tpmPath, err)
		}
	}()

	k, err := client.LoadCachedKey(rwc, tpmutil.Handle(*persistentHandle), nil)
	if err != nil {
		log.Printf("ERROR:  could not initialize Key: %v", err)
		return
	}

	r, err := sal.NewTPMCrypto(&sal.TPM{
		TpmDevice:      rwc,
		Key:            k,
		PublicCertFile: *publicCert,
	})
	if err != nil {
		log.Fatal(err)
	}

	// r, err := tls.LoadX509KeyPair("client-cert.pem", "client-key.pem")
	// if err != nil {
	// 	log.Fatal(err)
	// }

	dsn := fmt.Sprintf("user=%s database=%s sslmode=verify-ca", dbUser, dbName)
	config, err := pgx.ParseConfig(dsn)
	if err != nil {
		log.Printf("cloudsqlconn.NewDialer: %v", err)
		os.Exit(1)
	}

	tcrt, err := r.TLSCertificate()
	if err != nil {
		log.Printf("error getting tlscertificate: %v", err)
		os.Exit(1)
	}
	config.Password = *password
	config.Host = *host
	config.Port = 5432
	config.TLSConfig = &tls.Config{
		ServerName:   *sni,
		RootCAs:      caCertPool,
		Certificates: []tls.Certificate{tcrt},
	}

	sslKeyLogfile := os.Getenv("SSLKEYLOGFILE")
	if sslKeyLogfile != "" {
		var w *os.File
		w, err := os.OpenFile(sslKeyLogfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
		if err != nil {
			log.Fatalf("Could not create keylogger: ", err)
		}
		config.TLSConfig.KeyLogWriter = w
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
