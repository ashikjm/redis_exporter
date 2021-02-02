package exporter

import (
	"crypto/tls"
	"strings"

	"github.com/gomodule/redigo/redis"
	log "github.com/sirupsen/logrus"
)

func (e *Exporter) connectToRedis() (redis.Conn, error) {
	options := []redis.DialOption{
		redis.DialConnectTimeout(e.options.ConnectionTimeouts),
		redis.DialReadTimeout(e.options.ConnectionTimeouts),
		redis.DialWriteTimeout(e.options.ConnectionTimeouts),

		redis.DialTLSConfig(&tls.Config{
			InsecureSkipVerify: e.options.SkipTLSVerification,
			Certificates:       e.options.ClientCertificates,
			RootCAs:            e.options.CaCertificates,
		}),
	}

	if e.options.User != "" {
		options = append(options, redis.DialUsername(e.options.User))
	}

	if e.options.Password != "" {
		options = append(options, redis.DialPassword(e.options.Password))
	}

	uri := e.redisAddr

	if e.options.PasswordMap[uri] != "" {
		options = append(options, redis.DialPassword(e.options.PasswordMap[uri]))
	}

	isCluster := e.options.IsCluster

	if isCluster {

		if strings.Contains(uri, "://") {
			url, _ := url.Parse(uri)
			if url.Port() == "" {
				uri = url.Host + ":6379"
			} else {
				uri = url.Host
			}
		} else {
			if frags := strings.Split(uri, ":"); len(frags) != 2 {
				uri = uri + ":6379"
			}
		}

		log.Debugf("Creating cluster object")
		cluster := redisc.Cluster{
			StartupNodes: []string{uri},
			DialOptions:  options,
		}
		log.Debugf("Running refresh on cluster object")
		if err := cluster.Refresh(); err == nil {
			isCluster = true
		}

		log.Debugf("Creating redis connection object")
		conn, err := cluster.Dial()
		if err != nil {
			log.Debugf("Dial failed: %v", err)
		}

		c, err := redisc.RetryConn(conn, 10, 100*time.Millisecond)
		if err != nil {
			log.Debugf("RetryConn failed: %v", err)
		}

		return c, err
	}

	if !strings.Contains(uri, "://") {
		uri = "redis://" + uri
	}

	c, err := redis.DialURL(uri, options...)
	if err != nil {
		log.Debugf("DialURL() failed, err: %s", err)
		if frags := strings.Split(e.redisAddr, "://"); len(frags) == 2 {
			log.Debugf("Trying: Dial(): %s %s", frags[0], frags[1])
			c, err = redis.Dial(frags[0], frags[1], options...)
		} else {
			log.Debugf("Trying: Dial(): tcp %s", e.redisAddr)
			c, err = redis.Dial("tcp", e.redisAddr, options...)
		}
	}
	return c, err
}

func doRedisCmd(c redis.Conn, cmd string, args ...interface{}) (interface{}, error) {
	log.Debugf("c.Do() - running command: %s %s", cmd, args)
	res, err := c.Do(cmd, args...)
	if err != nil {
		log.Debugf("c.Do() - err: %s", err)
	}
	log.Debugf("c.Do() - done")
	return res, err
}
