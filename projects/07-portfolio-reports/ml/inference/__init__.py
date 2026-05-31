from inference.base import LLMBackend
from inference.llamacpp_backend import LlamaCppBackend
from inference.mock_backend import MockBackend
from inference.vllm_backend import VLLMBackend

__all__ = ["LLMBackend", "VLLMBackend", "LlamaCppBackend", "MockBackend"]


def build_backend(name: str, cfg):
    if name == "vllm":
        return VLLMBackend(cfg)
    if name == "llamacpp":
        return LlamaCppBackend(cfg)
    return MockBackend(cfg)
