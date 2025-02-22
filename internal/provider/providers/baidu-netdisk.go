package providers

import (
	"context"
	"fmt"
	"net/http"

	json "github.com/json-iterator/go"
	"github.com/synctv-org/synctv/internal/provider"
	"golang.org/x/oauth2"
)

// https://pan.baidu.com/union/apply
type BaiduNetDiskProvider struct {
	config oauth2.Config
}

func (p *BaiduNetDiskProvider) Init(c provider.Oauth2Option) {
	p.config.Scopes = []string{"basic", "netdisk"}
	p.config.Endpoint = oauth2.Endpoint{
		AuthURL:  "https://openapi.baidu.com/oauth/2.0/authorize",
		TokenURL: "https://openapi.baidu.com/oauth/2.0/token",
	}
	p.config.ClientID = c.ClientID
	p.config.ClientSecret = c.ClientSecret
	p.config.RedirectURL = c.RedirectURL
}

func (p *BaiduNetDiskProvider) Provider() provider.OAuth2Provider {
	return "baidu-netdisk"
}

func (p *BaiduNetDiskProvider) NewAuthURL(state string) string {
	return p.config.AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *BaiduNetDiskProvider) GetToken(ctx context.Context, code string) (*oauth2.Token, error) {
	return p.config.Exchange(ctx, code)
}

func (p *BaiduNetDiskProvider) RefreshToken(ctx context.Context, tk string) (*oauth2.Token, error) {
	return p.config.TokenSource(ctx, &oauth2.Token{RefreshToken: tk}).Token()
}

func (p *BaiduNetDiskProvider) GetUserInfo(ctx context.Context, tk *oauth2.Token) (*provider.UserInfo, error) {
	client := p.config.Client(ctx, tk)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("https://pan.baidu.com/rest/2.0/xpan/nas?method=uinfo&access_token=%s", tk.AccessToken), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	ui := baiduNetDiskProviderUserInfo{}
	err = json.NewDecoder(resp.Body).Decode(&ui)
	if err != nil {
		return nil, err
	}
	if ui.Errno != 0 {
		return nil, fmt.Errorf("baidu oauth2 get user info error: %s", ui.Errmsg)
	}
	return &provider.UserInfo{
		Username:       ui.BaiduName,
		ProviderUserID: ui.Uk,
	}, nil
}

type baiduNetDiskProviderUserInfo struct {
	BaiduName string `json:"baidu_name"`
	Errmsg    string `json:"errmsg"`
	Errno     int    `json:"errno"`
	Uk        uint   `json:"uk"`
}

func init() {
	RegisterProvider(new(BaiduNetDiskProvider))
}
