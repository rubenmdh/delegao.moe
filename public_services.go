package main

// PublicService is an entry in the public directory. It does not imply uptime.
type PublicService struct {
	Name, URL, Description, Icon string
}

func publicServices() []PublicService {
	return []PublicService{
		{Name: "safe", URL: "https://safe.delegao.moe/", Description: "File sharing", Icon: "img/services/safe.svg"},
		{Name: "paste", URL: "https://paste.delegao.moe/", Description: "Plain text", Icon: "img/services/paste.png"},
		{Name: "ip checker", URL: "https://ipv6.sx/", Description: "Your IP address", Icon: "img/services/ipv6.png"},
		{Name: "vid", URL: "https://vid.delegao.moe/", Description: "Piped · YouTube frontend", Icon: "img/services/vid.png"},
		{Name: "mumbweb", URL: "https://mumbweb.maduixa.re/", Description: "Web client for Mumble", Icon: "img/services/mumbweb.png"},
	}
}
