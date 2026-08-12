多云管理平台 - 单文件运行包

只有一个可执行文件需要关心（前端已内嵌，无需额外部署 Nginx）：
  - Windows (amd64): cloud-manager-windows-amd64.exe
  - Linux (arm64):   cloud-manager-linux-arm64

启动步骤:
  1. Windows: 双击运行 cloud-manager-windows-amd64.exe，或在命令行执行该文件
     Linux:   chmod +x cloud-manager-linux-arm64 && ./cloud-manager-linux-arm64
     默认以生产模式监听 0.0.0.0:8002
  2. (可选) 如需自定义端口/密钥等系统配置，在可执行文件旁的 data/ 目录下新建 config.json，例如:
       { "port": "9000", "jwt_secret": "your-secret", "aes_key": "your-32-byte-key!!" }
     字段留空则使用默认值；同名环境变量（PORT/HOST/DB_PATH/JWT_SECRET/AES_KEY/GIN_MODE）优先级更高。
  3. 浏览器访问 http://localhost:8002
     使用 admin / admin123 登录

首次运行会自动在 data/ 目录下创建 SQLite 数据库和管理员账号。
运行日志在 logs/ 目录下滚动记录（app.log，单文件 10MB，保留 10 份、30 天，自动压缩旧文件），
出问题时可直接查看该目录排查。
