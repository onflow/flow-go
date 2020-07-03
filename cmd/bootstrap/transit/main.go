package main

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"golang.org/x/crypto/nacl/box"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/dapperlabs/flow-go/model/bootstrap"
	"github.com/dapperlabs/flow-go/model/flow"
)

var (
	FilenameTransitKeyPub      = "transit-key.pub.%v"
	FilenameTransitKeyPriv     = "transit-key.priv.%v"
	FilenameRandomBeaconCipher = bootstrap.FilenameRandomBeaconPriv + ".%v.enc"
	flowBucket                 string
)

const fileMode = os.FileMode(0644)

var (

	// default files to upload for all role type
	filesToUpload = []string{
		bootstrap.PathNodeInfoPub,
	}

	// consensus node additionally will need the transit key (to securely transport DKG in phase 2)
	filesToUploadConsensus = FilenameTransitKeyPub

	// default folder to download for all role type
	folderToDownload = bootstrap.DirnamePublicBootstrap

	// consensus node additionally gets the random beacon file
	filesToDownloadConsensus = FilenameRandomBeaconCipher
)

func main() {

	var bootDir, keyDir, wrapID, role string
	var pull, push, prepare bool

	flag.StringVar(&bootDir, "d", "~/bootstrap", "The bootstrap directory containing your node-info files")
	flag.StringVar(&keyDir, "t", "", "Token provided by the Flow team to access the transit server")
	flag.BoolVar(&pull, "pull", false, "Fetch keys and metadata from the transit server")
	flag.BoolVar(&push, "push", false, "Upload public keys to the transit server")
	flag.BoolVar(&prepare, "prepare", false, "Generate transit keys for push step")
	flag.StringVar(&role, "role", "", `node role (can be "collection", "consensus", "execution", "verification" or "access")`)
	flag.StringVar(&wrapID, "x-server-wrap", "", "(Flow Team Use), wrap response keys for consensus node")
	flag.Parse()

	if role == "" {
		flag.Usage()
		log.Fatal("Node role must be specified")
	}

	flowRole, err := flow.ParseRole(role)
	if err != nil {
		flag.Usage()
		log.Fatalf(`unsupported role, allowed values: "collection"", "consensus", "execution", "verification" or "access""`)
	}

	// Wrap takes precedence, so we just do that first
	if wrapID != "" && flowRole == flow.RoleConsensus {
		log.Printf("Wrapping response for node %s\n", wrapID)
		err := wrapFile(bootDir, wrapID)
		if err != nil {
			log.Fatalf("Failed to wrap response: %s\n", err)
		}
		return
	}

	if optionsSelected(pull, push, prepare) != 1 {
		flag.Usage()
		log.Fatal("Exactly one of -pull, -push, or -prepare must be specified\n")
	}

	if !prepare && keyDir == "" {
		flag.Usage()
		log.Fatal("Access key, '-t', required for push and pull commands")
	}

	nodeID, err := fetchNodeID(bootDir)
	if err != nil {
		log.Fatalf("Could not determine node ID: %s\n", err)
	}

	// timeout remote operations
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	if push {
		runPush(ctx, bootDir, keyDir, nodeID, flowRole)
		return
	}

	if pull {
		runPull(ctx, bootDir, keyDir, nodeID, flowRole)
		return
	}

	if prepare {
		runPrepare(bootDir, nodeID, flowRole)
		return
	}
}

// Read the NodeID file to build other paths from
func fetchNodeID(bootDir string) (string, error) {
	path := filepath.Join(bootDir, bootstrap.PathNodeID)
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("Error reading file %s: %w", path, err)
	}

	return strings.TrimSpace(string(data)), nil
}

// Run the push process
// - create transit keypair (if the role type is Consensus)
// - upload files to GCS bucket
func runPush(ctx context.Context, bootDir, token, nodeId string, role flow.Role) {
	log.Println("Running push")

	if role == flow.RoleConsensus {
		err := generateKeys(bootDir, nodeId)
		if err != nil {
			log.Fatalf("Failed to push: %s", err)
		}
	}

	files := getFilesToUpload(role)

	for _, file := range files {
		err := bucketUpload(ctx, bootDir, fmt.Sprintf(file, nodeId), token)
		if err != nil {
			log.Fatalf("Failed to push: %s", err)
		}
	}
}

func runPull(ctx context.Context, bootDir, token, nodeId string, role flow.Role) {

	log.Println("Running pull")

	extraFiles := getAdditionalFilesToDownload(role, nodeId)

	var err error

	// download the public folder from the bucket and any additional files
	err = bucketDownload(ctx, bootDir, folderToDownload, bootstrap.DirnamePublicBootstrap, token, extraFiles...)
	if err != nil {
		log.Fatalf("Failed to pull: %s", err)
	}

	if role == flow.RoleConsensus {
		err = unwrapFile(bootDir, nodeId)
		if err != nil {
			log.Fatalf("Failed to pull: %s", err)
		}
	}

	rootFile := filepath.Join(bootDir, bootstrap.PathRootBlock)
	rootMD5, err := getFileMD5(rootFile)
	if err != nil {
		log.Fatalf("Failed to calculate md5 of %s: %v", rootFile, err)
	}
	log.Printf("MD5 of the root block is: %s\n", rootMD5)
}

// Run the prepare process
// - create transit keypair (if the role type is Consensus)
func runPrepare(bootdir, nodeId string, role flow.Role) {
	if role == flow.RoleConsensus {
		log.Println("creating transit-keys")
		err := generateKeys(bootdir, nodeId)
		if err != nil {
			log.Fatalf("Failed to prepare: %s", err)
		}
		return
	}
	log.Printf("no preparation needed for role: %s", role.String())
}

// generateKeys creates the transit keypair and writes them to disk for later
func generateKeys(bootDir, nodeId string) error {

	privPath := filepath.Join(bootDir, fmt.Sprintf(FilenameTransitKeyPriv, nodeId))
	pubPath := filepath.Join(bootDir, fmt.Sprintf(FilenameTransitKeyPub, nodeId))

	if fileExists(privPath) && fileExists(pubPath) {
		log.Print("transit-key-path priv & pub both exist, exiting")
		return nil
	}

	log.Print("Generating keypair")

	// Generate the keypair
	pub, priv, err := box.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("Failed to create keys: %w", err)
	}

	// Write private key file
	err = ioutil.WriteFile(privPath, priv[:], fileMode)
	if err != nil {
		return fmt.Errorf("Failed to write pivate key file: %w", err)
	}

	// Write public key file
	err = ioutil.WriteFile(pubPath, pub[:], fileMode)
	if err != nil {
		return fmt.Errorf("Failed to write public key file: %w", err)
	}

	return nil
}

func unwrapFile(bootDir, nodeId string) error {

	log.Print("Decrypting Random Beacon key")

	pubKeyPath := filepath.Join(bootDir, fmt.Sprintf(FilenameTransitKeyPub, nodeId))
	privKeyPath := filepath.Join(bootDir, fmt.Sprintf(FilenameTransitKeyPriv, nodeId))
	ciphertextPath := filepath.Join(bootDir, fmt.Sprintf(FilenameRandomBeaconCipher, nodeId))
	plaintextPath := filepath.Join(bootDir, fmt.Sprintf(bootstrap.PathRandomBeaconPriv, nodeId))

	ciphertext, err := ioutil.ReadFile(ciphertextPath)
	if err != nil {
		return fmt.Errorf("Failed to open ciphertext file %s: %w", ciphertextPath, err)
	}

	publicKey, err := ioutil.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("Failed to open public keyfile %s: %w", pubKeyPath, err)
	}

	privateKey, err := ioutil.ReadFile(privKeyPath)
	if err != nil {
		return fmt.Errorf("Failed to open private keyfile %s: %w", privKeyPath, err)
	}

	// NaCl is picky and wants its type to be exactly a [32]byte, but readfile reads a slice
	var pubKeyBytes, privKeyBytes [32]byte
	copy(pubKeyBytes[:], publicKey)
	copy(privKeyBytes[:], privateKey)

	plaintext := make([]byte, 0, len(ciphertext)-box.AnonymousOverhead)
	plaintext, ok := box.OpenAnonymous(plaintext, ciphertext, &pubKeyBytes, &privKeyBytes)
	if !ok {
		return fmt.Errorf("Failed to decrypt ciphertext: unknown error in NaCl")
	}

	err = ioutil.WriteFile(plaintextPath, plaintext, fileMode)
	if err != nil {
		return fmt.Errorf("Failed to write the decrypted file %s: %w", plaintextPath, err)
	}

	return nil
}

func wrapFile(bootDir, nodeId string) error {
	pubKeyPath := filepath.Join(bootDir, fmt.Sprintf(FilenameTransitKeyPub, nodeId))
	plaintextPath := filepath.Join(bootDir, fmt.Sprintf(bootstrap.PathRandomBeaconPriv, nodeId))
	ciphertextPath := filepath.Join(bootDir, fmt.Sprintf(FilenameRandomBeaconCipher, nodeId))

	plaintext, err := ioutil.ReadFile(plaintextPath)
	if err != nil {
		return fmt.Errorf("Failed to open plaintext file %s: %w", plaintextPath, err)
	}

	publicKey, err := ioutil.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("Faield to open public keyfile %s: %w", pubKeyPath, err)
	}

	var pubKeyBytes [32]byte
	copy(pubKeyBytes[:], publicKey)

	ciphertext := make([]byte, 0, len(plaintext)+box.AnonymousOverhead)

	ciphertext, err = box.SealAnonymous(ciphertext, plaintext, &pubKeyBytes, rand.Reader)
	if err != nil {
		return fmt.Errorf("Could not encrypt file: %w", err)
	}

	err = ioutil.WriteFile(ciphertextPath, ciphertext, fileMode)
	if err != nil {
		return fmt.Errorf("Error writing ciphertext: %w", err)
	}

	return nil
}

func bucketUpload(ctx context.Context, bootDir, filename, token string) error {

	gcsClient, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		return fmt.Errorf("Failed to initialize GCS client: %w", err)
	}
	defer gcsClient.Close()

	path := filepath.Join(bootDir, filename)
	log.Printf("Uploading %s\n", path)

	upload := gcsClient.Bucket(flowBucket).
		Object(filepath.Join(token, filename)).
		NewWriter(ctx)
	defer func() {
		err := upload.Close()
		if err != nil {
			log.Fatalf("Failed to close writer stream: %s\n", err)
		}
	}()

	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("Error opening upload file: %w", err)
	}
	defer file.Close()

	n, err := io.Copy(upload, file)
	if err != nil {
		return fmt.Errorf("Error uploading file: %w", err)
	}

	log.Printf("Uploaded %d bytes\n", n)

	return nil
}

// bucketDownload downloads all the files in srcFolder to bootDir/destFolder and additional fileNames to bootDir
func bucketDownload(ctx context.Context, bootDir, srcFolder, destFolder, token string, fileNames ...string) error {

	gcsClient, err := storage.NewClient(ctx, option.WithoutAuthentication())
	if err != nil {
		return fmt.Errorf("Failed to initialize GCS client: %w", err)
	}
	defer gcsClient.Close()

	bucket := gcsClient.Bucket(flowBucket)

	it := bucket.Objects(ctx, &storage.Query{
		Prefix: token + "/" + srcFolder + "/",
	})
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return fmt.Errorf("Bucket(%q).Objects(): %v", flowBucket, err)
		}

		err = bucketFileDownload(gcsClient, ctx, filepath.Join(bootDir, destFolder), attrs.Name)
		if err != nil {
			return err
		}
	}

	for _, file := range fileNames {
		objectName := filepath.Join(token, file)
		err = bucketFileDownload(gcsClient, ctx, bootDir, objectName)
		if err != nil {
			return err
		}
	}
	return nil
}

// bucketFileDownload downloads srcFile from storage to destFolder/srcFile on disk
func bucketFileDownload(gcsClient *storage.Client, ctx context.Context, destFolder, srcFile string) error {
	destFile := filepath.Base(srcFile)
	destPath := filepath.Join(destFolder, destFile)
	log.Printf("Downloading %s\n", destPath)

	download, err := gcsClient.Bucket(flowBucket).Object(srcFile).NewReader(ctx)
	if err != nil {
		return fmt.Errorf("error creating GCS object reader: %w", err)
	}
	defer download.Close()

	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("error creating download file: %w", err)
	}
	defer file.Close()

	_, err = io.Copy(file, download)
	if err != nil {
		return fmt.Errorf("error downloading file: %w", err)
	}
	return nil
}

func getFilesToUpload(role flow.Role) []string {
	switch role {
	case flow.RoleConsensus:
		return append(filesToUpload, filesToUploadConsensus)
	default:
		return filesToUpload
	}
}

func getAdditionalFilesToDownload(role flow.Role, nodeId string) []string {
	switch role {
	case flow.RoleConsensus:
		return []string{fmt.Sprintf(filesToDownloadConsensus, nodeId)}
	}
	return make([]string, 0)
}

func getFileMD5(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

func optionsSelected(options ...bool) int {
	n := 0
	for _, v := range options {
		if v {
			n++
		}
	}
	return n
}

func fileExists(filename string) bool {
	info, err := os.Stat(filename)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
