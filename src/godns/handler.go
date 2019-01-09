package main

import (
	"net"
	"time"

	"github.com/miekg/dns"
)



type GODNSHandler struct {
	resolver        *Resolver
	cache, negCache Cache
	hosts           Hosts
}

func NewHandler() *GODNSHandler {
    h := new(GODNSHandler)
    h.hosts = NewHosts(settings.Hosts, settings.Redis)
    
    resolvConfig := settings.ResolvConfig
	clientConfig, err := dns.ClientConfigFromFile(resolvConfig.ResolvFile)
	if err != nil {
		logger.Warn(":%s is not a valid resolv.conf file\n", resolvConfig.ResolvFile)
		logger.Error(err.Error())
		panic(err)
	}
	logger.Info("##### %+v \n", clientConfig)
	clientConfig.Timeout = resolvConfig.Timeout
	resolver := &Resolver{clientConfig}
	
	h.resolver = resolver
    
    return h
}

func NewHandler2() *GODNSHandler {

	var (
		clientConfig    *dns.ClientConfig
		cacheConfig     CacheSettings
		resolver        *Resolver
		cache, negCache Cache
	)

	resolvConfig := settings.ResolvConfig
	clientConfig, err := dns.ClientConfigFromFile(resolvConfig.ResolvFile)
	if err != nil {
		logger.Warn(":%s is not a valid resolv.conf file\n", resolvConfig.ResolvFile)
		logger.Error(err.Error())
		panic(err)
	}
	logger.Info("##### %+v \n", clientConfig)
	clientConfig.Timeout = resolvConfig.Timeout
	resolver = &Resolver{clientConfig}

	cacheConfig = settings.Cache
	switch cacheConfig.Backend {
	case "memory":
		cache = &MemoryCache{
			Backend:  make(map[string]Mesg, cacheConfig.Maxcount),
			Expire:   time.Duration(cacheConfig.Expire) * time.Second,
			Maxcount: cacheConfig.Maxcount,
		}
		negCache = &MemoryCache{
			Backend:  make(map[string]Mesg),
			Expire:   time.Duration(cacheConfig.Expire) * time.Second / 2,
			Maxcount: cacheConfig.Maxcount,
		}
	case "memcache":
		cache = NewMemcachedCache(
			settings.Memcache.Servers,
			int32(cacheConfig.Expire))
		negCache = NewMemcachedCache(
			settings.Memcache.Servers,
			int32(cacheConfig.Expire/2))
	case "redis":
		// cache = &MemoryCache{
		// 	Backend:    make(map[string]*dns.Msg),
		//  Expire:   time.Duration(cacheConfig.Expire) * time.Second,
		// 	Serializer: new(JsonSerializer),
		// 	Maxcount:   cacheConfig.Maxcount,
		// }
		panic("Redis cache backend not implement yet")
	default:
		logger.Error("Invalid cache backend %s", cacheConfig.Backend)
		panic("Invalid cache backend")
	}



    var hosts Hosts
    if ( true ) {
    	if settings.Hosts.Enable {
    		hosts = NewHosts(settings.Hosts, settings.Redis)
    	}
    	
    }

	return &GODNSHandler{resolver, cache, negCache, hosts}
}

func (h *GODNSHandler) do(Net string, w dns.ResponseWriter, req *dns.Msg) {
//	q := req.Question[0]
//	Q := Question{UnFqdn(q.Name), dns.TypeToString[q.Qtype], dns.ClassToString[q.Qclass]}

	var remote net.IP
	if Net == "tcp" {
		remote = w.RemoteAddr().(*net.TCPAddr).IP
	} else {
		remote = w.RemoteAddr().(*net.UDPAddr).IP
	}
	logger.Info("%s lookup　%s", remote, req.Question[0].Name)

    
    m := new(dns.Msg)
    m.SetReply(req)
    err := h.Get(Net, m, req)
    if err != nil {
        dns.HandleFailed(w, req)
    }

	w.WriteMsg(m)

}

func (h *GODNSHandler) DoTCP(w dns.ResponseWriter, req *dns.Msg) {
	h.do("tcp", w, req)
}

func (h *GODNSHandler) DoUDP(w dns.ResponseWriter, req *dns.Msg) {
	h.do("udp", w, req)
}

func (h *GODNSHandler) isIPQuery(q dns.Question) uint16 {
	if q.Qclass != dns.ClassINET {
	    return dns.TypeNone
	}

	switch q.Qtype {
	case dns.TypeA:
	    return dns.TypeA
	case dns.TypeAAAA:
	    return dns.TypeAAAA
	default:
		return dns.TypeNone
	}
}



func (h *GODNSHandler) Get(Net string, msg *dns.Msg, req *dns.Msg) (error) {
    var records []string
    var err error
    
    fqname := req.Question[0].Name
    qname := UnFqdn(fqname)
    
    logger.Info("-------- Net:%s serch-domain:%s qname:%s", Net, fqname, qname)
    
    records, err = h.hosts.Get(qname)
//    if err != nil {
//        logger.Error(err.Error())
//        return err
//    }
    
    for _, record := range records {
	    ip := net.ParseIP(record)
	    frecord := dns.Fqdn(record)
	    logger.Info("## %s %s", record, qname)
	    
	    if ip != nil {
	        rr_header := dns.RR_Header {
		        Name:	fqname,
		        Rrtype:	dns.TypeA,
		        Class:	dns.ClassINET,
		        Ttl:    settings.Hosts.TTL,
		    }
		    rr := &dns.A{rr_header, ip}
		    
		    msg.Answer = append(msg.Answer, rr)
		    
		    return nil
	    } else {
	        ////////////////////////////////
    	    rr_header := dns.RR_Header {
    	        Name:	fqname,
    	        Rrtype:	dns.TypeCNAME,
    	        Class:	dns.ClassINET,
    	        Ttl:    settings.Hosts.TTL,
    	    }
    	    rr := &dns.CNAME{rr_header, frecord}
    	    
    	    msg.Answer = append(msg.Answer, rr)
    	    
    	    nreq := new(dns.Msg)
    	    nreq.Id = req.Id
    	    Q := dns.Question{
	            Name : rr.Target, 
	            Qtype : dns.TypeA, 
	            Qclass: dns.ClassINET,
    	    }
    	    
    	    nreq.Question = append(nreq.Question, Q)
    	    
    	    h.Get(Net, msg, nreq)
    	    return nil
	    }
	}
    
    logger.Info("++@@@  req:%+v\n \n", req)
    msg2, err2 := h.resolver.Lookup(Net, req)
    logger.Info("--@@@  req:%+v\n msg:%+v\n", req, msg2)
	if err2 != nil {
		logger.Warn("Resolve query error %s", err2.Error())

		return err
	}
	
	for _,a := range msg2.Answer {
    	msg.Answer = append(msg.Answer, a)
	}
    
    return nil
}



//cut like: baidu.com. -> baidu.com
func UnFqdn(s string) string {
	if dns.IsFqdn(s) {
		return s[:len(s)-1]
	}
	return s
}
