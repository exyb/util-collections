单纯的增量镜像工具
- go  不可用 (希望基于oci工具和镜像仓库manifest, 生成增量容器镜像, 兼容 docker/oci manifest. 但是目前没有处理好manifest index, 无法完善的处理多架构)
- shell 可以使用 用脚本生成增量容器镜像
