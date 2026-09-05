all: clean
	@ echo "    * Building listener_papershell_http plugin"
	@ mkdir dist
	@ cp config.yaml ax_config.axs ./dist/
	@ GOEXPERIMENT=jsonv2,greenteagc go build -buildmode=plugin -ldflags="-s -w" -o ./dist/papershell_http.so pl_main.go pl_transport.go pl_utils.go
	@ echo "      done..."

clean:
	@ rm -rf dist
