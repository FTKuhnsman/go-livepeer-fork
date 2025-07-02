package main

import (
	pc "github.com/livepeer/go-livepeer/eth/rpcpool"
)

func main() {
	pc.GenerateEthClient(pc.EthclientPkg, "ethclient.go", pc.EthClientWrapperTemplate, "ethclient_generated.go")

	pc.GenerateEthClient(pc.RpcclientPkg, "client.go", pc.RpcClientWrapperTemplate, "rpcclient_generated.go")

}
