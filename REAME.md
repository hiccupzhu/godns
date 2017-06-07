## 项目说明

项目主要是依赖 [https://github.com/miekg/dns](https://github.com/miekg/dns)（一个GO语言写的底层dns解析库）写的一个dns解析服务；

目前只支持A记录解析；

##  安装

    go get github.com/kenshinx/godns 
    #注意安装过程中使用到了其它第三方库，需要连接vpn翻墙下载

## 编译运行

    go build -o godns
    ./godns -c godns.conf