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

	"cloud.google.com/go/storage"
	"golang.org/x/crypto/nacl/box"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	"github.com/onflow/flow-go/cmd/bootstrap/build"
	"github.com/onflow/flow-go/ledger/complete/wal"
	"github.com/onflow/flow-go/model/bootstrap"
	"github.com/onflow/flow-go/model/flow"
	utilsio "github.com/onflow/flow-go/utils/io"
)

var (
	FilenameTransitKeyPub      = "transit-key.pub.%v"
	FilenameTransitKeyPriv     = "transit-key.priv.%v"
	FilenameRandomBeaconCipher = bootstrap.FilenameRandomBeaconPriv + ".%v.enc"
	flowBucket                 string
	commit                     = build.Commit()
	semver                     = build.Semver()
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
	var version, pull, push, prepare bool

	flag.BoolVar(&version, "v", false, "View version and commit information")
	flag.StringVar(&bootDir, "d", "~/bootstrap", "The bootstrap directory containing your node-info files")
	flag.StringVar(&keyDir, "t", "", "Token provided by the Flow team to access the transit server")
	flag.BoolVar(&pull, "pull", false, "Fetch keys and metadata from the transit server")
	flag.BoolVar(&push, "push", false, "Upload public keys to the transit server")
	flag.BoolVar(&prepare, "prepare", false, "Generate transit keys for push step")
	flag.StringVar(&role, "role", "", `node role (can be "collection", "consensus", "execution", "verification" or "access")`)
	flag.StringVar(&wrapID, "x-server-wrap", "", "(Flow Team Use), wrap response keys for consensus node")
	flag.StringVar(&flowBucket, "flow-bucket", "flow-genesis-bootstrap", "Storage for the transit server")
	flag.Parse()

	// always print version information
	printVersion()
	if version {
		return
	}

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

	// use a context without timeout since certain files are large (e.g. root.checkpoint) and depending on the
	// network connection may need a lot of time to download
	ctx, cancel := context.WithCancel(context.Background())
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
	data, err := utilsio.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("Error reading file %s: %w", path, err)
	}

	return strings.TrimSpace(string(data)), nil
}

// Print the version and commit id
func printVersion() {
	// Print version/commit strings if they are known
	if build.IsDefined(semver) {
		fmt.Printf("Transit script Version: %s\n", semver)
	}
	if build.IsDefined(commit) {
		fmt.Printf("Transit script Commit: %s\n", commit)
	}
	// If no version info is known print a message to indicate this.
	if !build.IsDefined(semver) && !build.IsDefined(commit) {
		fmt.Printf("Transit script version information unknown\n")
	}
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

	if role == flow.RoleExecution {
		// for an execution node, move the root.checkpoint file from <bootstrap folder>/public-root-information dir to
		// <bootstrap folder>/execution-state dir

		// root.checkpoint is downloaded to <bootstrap folder>/public-root-information after a pull
		rootCheckpointSrc := filepath.Join(bootDir, bootstrap.DirnamePublicBootstrap, wal.RootCheckpointFilename)

		rootCheckpointDst := filepath.Join(bootDir, bootstrap.PathRootCheckpoint)

		log.Printf("Moving %s to %s \n", rootCheckpointSrc, rootCheckpointDst)

		err := moveFile(rootCheckpointSrc, rootCheckpointDst)
		if err != nil {
			log.Fatalf("Failed to move root.checkpoint from %s to %s: %s", rootCheckpointSrc, rootCheckpointDst, err)
		}
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

	ciphertext, err := utilsio.ReadFile(ciphertextPath)
	if err != nil {
		return fmt.Errorf("Failed to open ciphertext file %s: %w", ciphertextPath, err)
	}

	publicKey, err := utilsio.ReadFile(pubKeyPath)
	if err != nil {
		return fmt.Errorf("Failed to open public keyfile %s: %w", pubKeyPath, err)
	}

	privateKey, err := utilsio.ReadFile(privKeyPath)
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
		return fmt.Errorf("Failed to decrypt random beacon key using private key from file: %s", privKeyPath)
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

	plaintext, err := utilsio.ReadFile(plaintextPath)
	if err != nil {
		return fmt.Errorf("Failed to open plaintext file %s: %w", plaintextPath, err)
	}

	publicKey, err := utilsio.ReadFile(pubKeyPath)
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

// moveFile moves a file from source to destination where src and dst are full paths including the filename
func moveFile(src, dst string) error {

	// check if source file exist
	if !fileExists(src) {
		return fmt.Errorf("file not found: %s", src)
	}

	// create the destination dir if it does not exist
	destinationDir := filepath.Dir(dst)
	err := os.MkdirAll(destinationDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory %s: %w", destinationDir, err)
	}

	// first, try renaming the file
	err = os.Rename(src, dst)
	if err == nil {
		// if renaming works, we are done
		return nil
	}

	// renaming may fail if the destination dir is on a different disk, in that case we do a copy followed by remove
	// open the source file
	source, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", src, err)
	}

	// create the destination file
	destination, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", dst, err)
	}
	defer destination.Close()

	// copy the source file to the destination file
	_, err = io.Copy(destination, source)
	if err != nil {
		errorStr := err.Error()
		closeErr := source.Close()
		if closeErr != nil {
			errorStr = fmt.Sprintf("%s, %s", errorStr, closeErr)
		}
		return fmt.Errorf("failed to copy file %s to %s: %s", src, dst, errorStr)
	}

	// close the source file
	err = source.Close()
	if err != nil {
		return fmt.Errorf("failed to close source file %s: %w", src, err)
	}

	// flush the destination file
	err = destination.Sync()
	if err != nil {
		return fmt.Errorf("failed to copy file %s to %s: %w", src, dst, err)
	}

	// read the source file permissions
	si, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("failed to get file information %s: %w", src, err)
	}

	// set the same permissions on the destination file
	err = os.Chmod(dst, si.Mode())
	if err != nil {
		return fmt.Errorf("failed to set permisson on file %s: %w", dst, err)
	}

	// delete the source file
	err = os.Remove(src)
	if err != nil {
		return fmt.Errorf("failed removing original file: %s", err)
	}

	return nil
}
