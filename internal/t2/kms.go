package t2

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type SignedLockProof struct { LockProof LockProof; PublicKeyB64 string; SignatureB64 string }

func SignLockProof(p LockProof, privateKey ed25519.PrivateKey)(SignedLockProof,error){sig:=ed25519.Sign(privateKey,MustJSON(p));pub:=privateKey.Public().(ed25519.PublicKey);return SignedLockProof{LockProof:p,PublicKeyB64:base64.StdEncoding.EncodeToString(pub),SignatureB64:base64.StdEncoding.EncodeToString(sig)},nil}
func VerifyLockProof(s SignedLockProof)error{pub,err:=base64.StdEncoding.DecodeString(s.PublicKeyB64);if err!=nil{return err};sig,err:=base64.StdEncoding.DecodeString(s.SignatureB64);if err!=nil{return err};if len(pub)!=ed25519.PublicKeySize||len(sig)!=ed25519.SignatureSize{return errors.New("invalid lock proof size")};if !ed25519.Verify(ed25519.PublicKey(pub),MustJSON(s.LockProof),sig){return errors.New("invalid lock proof signature")};return nil}

func HashDirectory(path string) (string,error) {
	entries,err:=os.ReadDir(path);if err!=nil{return "",err}
	type item struct{name string;mode os.FileMode;size int64;sum string}
	items:=make([]item,0,len(entries))
	for _,e:=range entries{
		info,err:=e.Info();if err!=nil{return "",err}
		sum:=""
		if info.Mode().IsRegular(){b,err:=os.ReadFile(filepath.Join(path,e.Name()));if err!=nil{return "",err};sum=SHA256Bytes(b)}
		items=append(items,item{name:e.Name(),mode:info.Mode(),size:info.Size(),sum:sum})
	}
	return SHA256Bytes(MustJSON(items)),nil
}

func RequireEmptyDirectory(path string) error {
	entries,err:=os.ReadDir(path);if err!=nil{return err}
	if len(entries)!=0{return errors.New("fresh-state directory is not empty")}
	return nil
}

func BuildLockProof(studyID,preregHash,phase2Dir,freshDir string,budget ResourceBudget)(LockProof,error){
	if studyID==""||preregHash==""{return LockProof{},errors.New("study ID and prereg hash required")}
	if err:=RequireEmptyDirectory(phase2Dir);err!=nil{return LockProof{},fmt.Errorf("phase-2 purge not established: %w",err)}
	if err:=RequireEmptyDirectory(freshDir);err!=nil{return LockProof{},fmt.Errorf("fresh state not empty: %w",err)}
	phaseHash,err:=HashDirectory(phase2Dir);if err!=nil{return LockProof{},err}
	freshHash,err:=HashDirectory(freshDir);if err!=nil{return LockProof{},err}
	nonceRaw:=make([]byte,32);if _,err:=rand.Read(nonceRaw);err!=nil{return LockProof{},err}
	return LockProof{StudyID:studyID,PreregHash:preregHash,FreshStateHash:freshHash,ResourceHash:SHA256Bytes(MustJSON(budget)),Phase2PurgeHash:phaseHash,Nonce:base64.RawURLEncoding.EncodeToString(nonceRaw),IssuedAtUnix:time.Now().Unix()},nil
}

type RemoteKMS struct { BaseURL string; BearerToken string; Client *http.Client }
type kmsStoreRequest struct { StudyID string; ValidateCipherHash string; KeyB64 string }
type kmsReleaseRequest struct { Signed SignedLockProof; ValidateCipherHash string }
type kmsReleaseResponse struct { KeyB64 string }

func NewRemoteKMSFromEnv()(RemoteKMS,error){url:=os.Getenv("T2_KMS_URL");token:=os.Getenv("T2_KMS_BEARER_TOKEN");if url==""||token==""{return RemoteKMS{},errors.New("T2_KMS_URL and T2_KMS_BEARER_TOKEN are required")};if !strings.HasPrefix(url,"https://"){return RemoteKMS{},errors.New("T2_KMS_URL must use HTTPS")};tlsCfg:=&tls.Config{MinVersion:tls.VersionTLS13};if ca:=os.Getenv("T2_KMS_CA");ca!=""{b,err:=os.ReadFile(ca);if err!=nil{return RemoteKMS{},err};roots:=x509.NewCertPool();if !roots.AppendCertsFromPEM(b){return RemoteKMS{},errors.New("invalid KMS CA")};tlsCfg.RootCAs=roots};if cert,key:=os.Getenv("T2_KMS_CLIENT_CERT"),os.Getenv("T2_KMS_CLIENT_KEY");cert!=""&&key!=""{pair,err:=tls.LoadX509KeyPair(cert,key);if err!=nil{return RemoteKMS{},err};tlsCfg.Certificates=[]tls.Certificate{pair}};return RemoteKMS{BaseURL:strings.TrimRight(url,"/"),BearerToken:token,Client:&http.Client{Transport:&http.Transport{TLSClientConfig:tlsCfg},Timeout:20*time.Second}},nil}

func(k RemoteKMS) StoreValidateKey(studyID,hash string,key []byte)error{if len(key)!=32{return errors.New("AES-256 key required")};return k.post("/v1/t2/keys",kmsStoreRequest{StudyID:studyID,ValidateCipherHash:hash,KeyB64:base64.StdEncoding.EncodeToString(key)},nil)}
func(k RemoteKMS) ReleaseValidateKey(s SignedLockProof,hash string)([]byte,error){if err:=VerifyLockProof(s);err!=nil{return nil,err};var out kmsReleaseResponse;if err:=k.post("/v1/t2/release",kmsReleaseRequest{Signed:s,ValidateCipherHash:hash},&out);err!=nil{return nil,err};key,err:=base64.StdEncoding.DecodeString(out.KeyB64);if err!=nil{return nil,err};if len(key)!=32{return nil,errors.New("KMS returned invalid key")};return key,nil}

func(k RemoteKMS) post(path string,payload any,out any)error{if k.Client==nil{return errors.New("nil KMS client")};b,err:=json.Marshal(payload);if err!=nil{return err};req,err:=http.NewRequest(http.MethodPost,k.BaseURL+path,bytes.NewReader(b));if err!=nil{return err};req.Header.Set("Content-Type","application/json");req.Header.Set("Authorization","Bearer "+k.BearerToken);resp,err:=k.Client.Do(req);if err!=nil{return err};defer resp.Body.Close();if resp.StatusCode<200||resp.StatusCode>=300{return fmt.Errorf("remote KMS HTTP status %d",resp.StatusCode)};if out!=nil{return json.NewDecoder(resp.Body).Decode(out)};return nil}
