all: clean
	@ echo "    * Building agent_papershell plugin"
	@ mkdir dist
	@ cp config.yaml ax_config.axs ./dist/
	@ cp -r src_papershell ./dist/
	@ GOEXPERIMENT=jsonv2,greenteagc go build -buildmode=plugin -ldflags="-s -w" -o ./dist/agent_papershell.so pl_main.go pl_agent.go pl_packer.go pl_utils.go
	@ echo "      done..."

#	@ echo "    * Building agent"
# 	PRE BUILDING OBJECTS
#	@ echo "      done..."

clean:
	@ rm -rf dist