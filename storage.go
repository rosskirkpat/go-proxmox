package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	scp "github.com/bramvdbogaerde/go-scp"
	"github.com/bramvdbogaerde/go-scp/auth"
	"golang.org/x/crypto/ssh"
	"os"
	"path/filepath"
)

var validContent = map[string]struct{}{
	"iso":     struct{}{},
	"vztmpl":  struct{}{},
	"snippet": struct{}{},
}

func (s *Storage) Upload(content, file string) (*Task, error) {
	if _, ok := validContent[content]; !ok {
		return nil, fmt.Errorf("only iso and vztmpl allowed")
	}

	stat, err := os.Stat(file)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("file is a directory %s", file)
	}

	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var upid UPID
	if err := s.client.Upload(fmt.Sprintf("/nodes/%s/storage/%s/upload", s.Node, s.Name),
		map[string]string{"content": content}, f, &upid); err != nil {
		return nil, err
	}

	return NewTask(upid, s.client), nil
}

func (s *Storage) DownloadURL(content, filename, url string) (*Task, error) {
	if _, ok := validContent[content]; !ok {
		return nil, fmt.Errorf("only iso and vztmpl allowed")
	}

	var upid UPID
	s.client.Post(fmt.Sprintf("/nodes/%s/storage/%s/download-url", s.Node, s.Name), map[string]string{
		"content":  content,
		"filename": filename,
		"url":      url,
	}, &upid)
	return NewTask(upid, s.client), nil
}

func (s *Storage) ISO(name string) (iso *ISO, err error) {
	err = s.client.Get(fmt.Sprintf("/nodes/%s/storage/%s/content/%s:%s/%s", s.Node, s.Name, s.Name, "iso", name), &iso)
	if err != nil {
		return nil, err
	}

	iso.client = s.client
	iso.Node = s.Node
	iso.Storage = s.Name
	if iso.VolID == "" {
		iso.VolID = fmt.Sprintf("%s:iso/%s", iso.Storage, name)
	}
	return
}

func (s *Storage) Snippet(name string) (snippet *Snippet, err error) {
	volumeId := fmt.Sprintf("snippets:snippets/%s", name)
	// nodes/promoxx-nuc1/storage/snippets/content/snippets:snippets/ubuntu-cc.yaml

	vol := &StorageVolume{}
	err = s.client.Get(fmt.Sprintf("/nodes/%s/storage/snippets/content/%s", s.Node, volumeId), &vol)
	if err != nil {
		return nil, err
	}
	snippet.Node = s.Node
	snippet.Path = vol.Path
	snippet.Storage = s.Name
	snippet.Size = StringOrUint64(vol.Size)
	snippet.Used = StringOrUint64(vol.Used)
	snippet.client = s.client
	snippet.VolID = volumeId

	// fully built snippet url:
	// https://192.168.1.21:8006/api2/json/nodes/promoxx-nuc1/storage/snippets/content/snippets:snippets/ubuntu-cc.yaml
	snippet.URL = fmt.Sprintf("%s%s", s.client.baseURL, fmt.Sprintf("/nodes/%s/storage/snippets/content/%s", s.Node, volumeId))

	return snippet, nil
}

func (s *Storage) VzTmpl(name string) (vztmpl *VzTmpl, err error) {
	err = s.client.Get(fmt.Sprintf("/nodes/%s/storage/%s/content/%s:%s/%s", s.Node, s.Name, s.Name, "vztmpl", name), &vztmpl)
	if err != nil {
		return nil, err
	}

	vztmpl.client = s.client
	vztmpl.Node = s.Node
	vztmpl.Storage = s.Name
	if vztmpl.VolID == "" {
		vztmpl.VolID = fmt.Sprintf("%s:vztmpl/%s", vztmpl.Storage, name)
	}
	return
}

func (s *Storage) Backup(name string) (backup *Backup, err error) {
	err = s.client.Get(fmt.Sprintf("/nodes/%s/storage/%s/content/%s:%s/%s", s.Node, s.Name, s.Name, "backup", name), &backup)
	if err != nil {
		return nil, err
	}

	backup.client = s.client
	backup.Node = s.Node
	backup.Storage = s.Name
	return
}

func (v *VzTmpl) Delete() (*Task, error) {
	return deleteVolume(v.client, v.Node, v.Storage, v.VolID, v.Path, "vztmpl")
}

func (b *Backup) Delete() (*Task, error) {
	return deleteVolume(b.client, b.Node, b.Storage, b.VolID, b.Path, "backup")
}

func (i *ISO) Delete() (*Task, error) {
	return deleteVolume(i.client, i.Node, i.Storage, i.VolID, i.Path, "iso")
}

func deleteVolume(c *Client, n, s, v, p, t string) (*Task, error) {
	var upid UPID
	if v == "" && p == "" {
		return nil, fmt.Errorf("volid or path required for a delete")
	}

	if v == "" {
		// volid not returned in the volume endpoints, need to generate
		v = fmt.Sprintf("%s:%s/%s", s, t, filepath.Base(p))
	}

	err := c.Delete(fmt.Sprintf("/nodes/%s/storage/%s/content/%s", n, s, v), &upid)
	return NewTask(upid, c), err
}

func (s *Storage) newSnippetsStorageDirectory() error {
	snippetsStorage := &Storage{}

	newSnippetDirectory := map[string]string{
		"storage":       "snippet",
		"path":          "/var/lib/vz",
		"content":       "snippets",
		"nodes":         "",
		"shared":        "1",
		"type":          "dir",
		"disable":       "0",
		"prune-backups": "keep-all=1",
	}
	snippetPost, err := json.Marshal(newSnippetDirectory)
	if err != nil {
		return err
	}
	newSnippetsStorage := &Storage{}

	err = json.Unmarshal(snippetPost, newSnippetsStorage)
	if err != nil {
		return err
	}

	return s.client.Post("/storage/snippets", newSnippetsStorage, snippetsStorage)
}

func (s *Storage) ScpUpload(name, contentType, localPath string) error {
	node, err := s.client.Node(s.Node)
	if err != nil {
		return err
	}
	var nodeIp string
	c, _ := s.client.Cluster()
	for _, ns := range c.Nodes {
		if ns.Name == node.Name {
			nodeIp = ns.IP
			break
		}
		continue
	}

	clientConfig, err := auth.PrivateKey(s.client.credentials.SshUsername, s.client.credentials.SshPrivateKeyPath, ssh.InsecureIgnoreHostKey())
	if err != nil {
		return err
	}

	scpClient := scp.NewClient(fmt.Sprintf("%s:%v", nodeIp, s.client.credentials.SshPort), &clientConfig)
	err = scpClient.Connect()
	if err != nil {
		return err
	}
	defer scpClient.Close()

	f, err := os.Open(localPath)
	if err != nil {
		return err
	}
	defer f.Close()

	var remotePath string
	switch contentType {
	case "snippet":
		{
			snippetsStorage := &Storage{}
			// check if the snippets storage directory already exists and create it if it doesn't
			if err = s.client.Get("/storage/snippets", snippetsStorage); err != nil || snippetsStorage == nil {
				err = s.newSnippetsStorageDirectory()
				if err != nil {
					return err
				}
			}

			volumeId := fmt.Sprintf("snippets:snippets/%s", name)

			vol := &StorageVolume{}
			// /nodes/promoxx-nuc1/storage/snippets/content/snippets:snippets/ubuntu-cc.yaml
			err = s.client.Get(fmt.Sprintf("/nodes/%s/storage/snippets/content/%s", s.Node, volumeId), &vol)
			if err != nil {
				return err
			}
			// local proxmox filesystem path: "/var/lib/vz/snippets/ubuntu-cc.yaml"
			remotePath = vol.Path

		}
	case "iso":
		{
			// TODO implement me
			return nil
		}
	case "vztmpl":
		{
			// TODO implement me
			return nil
		}
	}

	return scpClient.CopyFromFile(context.Background(), *f, remotePath, "0655")

}
