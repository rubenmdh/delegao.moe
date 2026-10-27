package main

// PublicService is an entry in the public directory. It does not imply uptime.
type PublicService struct {
	Name, URL, Description string
}

func publicServices() []PublicService {
	return []PublicService{
		{Name: "safe", URL: "https://safe.delegao.moe/", Description: "File sharing"},
		{Name: "paste", URL: "https://paste.delegao.moe/", Description: "Plain text"},
		{Name: "ip checker", URL: "https://ipv6.sx/", Description: "Your IP address"},
		{Name: "piped", URL: "https://vid.delegao.moe/", Description: "YouTube frontend"},
	}
}
