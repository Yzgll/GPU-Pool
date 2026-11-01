GPU Pool

私有 GPU 算力共享平台。

## Project Goals

- GPU 节点自动注册
- GPU 资源监控
- 多节点资源池管理
- 分布式大模型推理
- 统一 Web 管理界面
- OpenAI 兼容 API

## Technology Stack

- Backend: Go + Gin
- Agent: Go
- Database: PostgreSQL
- Frontend: Vue 3 + TypeScript
- GPU Monitor: NVIDIA-SMI / NVML
- Inference Engine: vLLM
- Deployment: Docker

## Project Structure

- server: Coordinator
- agent: GPU Worker Agent
- frontend: Web Console
- inference: Distributed Inference
- deploy: Deployment Configuration
- docs: Project Documentation

## Development Plan

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md).
