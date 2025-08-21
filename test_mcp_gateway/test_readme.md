当前项目 mcp_srv_mgr 是一个实现了 http api 服务和3种MCP协议的可用于实现linux操作系统服务启动管理的程序。

我希望能将这个程序接入到 https://github.com/AmoyLab/Unla 这个项目中去，这是一个可实现 http api 和3种mcp协议代理的服务，将这些协议统一代理并以同样的 mcp 协议提供给 ai 大模型作为 mcp 工具。

请使用 context-7 详细参考 unla 这个 mcp gateway 服务的文档，然后实现当前 mcp_srv_mgr 的 http api、3种MCP协议接口的接入，并对所有的接入方式进行测试。

请将所有的接入测试所需的代码、文档都放在 test_mcp_gateway 目录下。

我目前已经编译好了 unla 的 mcp-gateway 服务程序的二进制代码，放在 test_mcp_gateway 目录下，我只希望使用 unla 的 mcp-gateway，并不使用它的 apiserver。

后端测试数据库请使用 mysql，当前这个数据库位于本机的 3311 端口，用户名 root 密码 nov24feb11；如果需要使用redis，可以使用本机的 6379 端口的redis，没有密码。

如果你还有其他基础组件需要，请告诉我，让我来操作。