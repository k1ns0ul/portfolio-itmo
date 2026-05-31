from __future__ import annotations

import logging
from dataclasses import dataclass, field
from pathlib import Path

import joblib
import numpy as np
import torch
from sklearn.preprocessing import StandardScaler
from torch import nn
from torch.utils.data import DataLoader, TensorDataset

log = logging.getLogger(__name__)


@dataclass
class AutoencoderParams:
    input_dim: int = 8
    hidden_dims: tuple[int, ...] = (32, 16, 8)
    epochs: int = 100
    batch_size: int = 512
    lr: float = 1e-3
    weight_decay: float = 1e-5
    patience: int = 15
    val_split: float = 0.1
    threshold_percentile: float = 99.0
    random_state: int = 42
    device: str = "cpu"


class _Net(nn.Module):
    def __init__(self, input_dim: int, hidden: tuple[int, ...]) -> None:
        super().__init__()
        enc_layers: list[nn.Module] = []
        prev = input_dim
        for h in hidden:
            enc_layers.append(nn.Linear(prev, h))
            enc_layers.append(nn.BatchNorm1d(h))
            enc_layers.append(nn.LeakyReLU(0.1, inplace=True))
            prev = h
        self.encoder = nn.Sequential(*enc_layers)

        dec_layers: list[nn.Module] = []
        rev = list(hidden)[::-1]
        for h in rev[1:]:
            dec_layers.append(nn.Linear(prev, h))
            dec_layers.append(nn.BatchNorm1d(h))
            dec_layers.append(nn.LeakyReLU(0.1, inplace=True))
            prev = h
        dec_layers.append(nn.Linear(prev, input_dim))
        self.decoder = nn.Sequential(*dec_layers)

    def forward(self, x: torch.Tensor) -> torch.Tensor:
        return self.decoder(self.encoder(x))


@dataclass
class AutoencoderModel:
    params: AutoencoderParams = field(default_factory=AutoencoderParams)
    net: _Net | None = None
    scaler: StandardScaler | None = None
    threshold: float = 0.0
    training_errors: np.ndarray = field(default_factory=lambda: np.zeros(0, dtype=np.float32))

    def fit(self, X: np.ndarray) -> dict[str, float]:
        if X.size == 0:
            raise ValueError("empty training set")
        torch.manual_seed(self.params.random_state)
        device = torch.device(self.params.device)

        rng = np.random.default_rng(self.params.random_state)
        idx = np.arange(len(X))
        rng.shuffle(idx)
        val_count = max(1, int(len(idx) * self.params.val_split))
        val_idx, train_idx = idx[:val_count], idx[val_count:]

        X_train = X[train_idx]
        X_val = X[val_idx]

        self.scaler = StandardScaler()
        self.scaler.fit(X_train)
        Xs_train = self.scaler.transform(X_train).astype(np.float32)
        Xs_val = self.scaler.transform(X_val).astype(np.float32)

        train_loader = DataLoader(
            TensorDataset(torch.from_numpy(Xs_train)),
            batch_size=self.params.batch_size, shuffle=True, drop_last=False,
        )
        val_loader = DataLoader(
            TensorDataset(torch.from_numpy(Xs_val)),
            batch_size=self.params.batch_size, shuffle=False,
        )

        self.params.input_dim = Xs_train.shape[1]
        self.net = _Net(self.params.input_dim, self.params.hidden_dims).to(device)
        opt = torch.optim.Adam(self.net.parameters(), lr=self.params.lr, weight_decay=self.params.weight_decay)
        criterion = nn.MSELoss()

        best_val = float("inf")
        patience_left = self.params.patience
        best_state = {k: v.detach().clone() for k, v in self.net.state_dict().items()}

        for epoch in range(self.params.epochs):
            self.net.train()
            train_loss = 0.0
            seen = 0
            for (batch,) in train_loader:
                batch = batch.to(device)
                opt.zero_grad()
                out = self.net(batch)
                loss = criterion(out, batch)
                loss.backward()
                opt.step()
                train_loss += float(loss.item()) * batch.size(0)
                seen += batch.size(0)
            train_loss /= max(1, seen)

            val_loss = self._eval_loss(val_loader, criterion, device)
            log.info("ae epoch=%d train=%.5f val=%.5f", epoch + 1, train_loss, val_loss)

            if val_loss < best_val - 1e-5:
                best_val = val_loss
                patience_left = self.params.patience
                best_state = {k: v.detach().clone() for k, v in self.net.state_dict().items()}
            else:
                patience_left -= 1
                if patience_left <= 0:
                    log.info("ae early stop at epoch %d", epoch + 1)
                    break

        self.net.load_state_dict(best_state)
        self.net.eval()

        errors = self._reconstruction_errors(Xs_train)
        self.training_errors = errors.astype(np.float32)
        self.threshold = float(np.percentile(errors, self.params.threshold_percentile))
        return {
            "best_val_loss": float(best_val),
            "threshold": self.threshold,
            "mean_error": float(errors.mean()),
        }

    def predict(self, X: np.ndarray) -> np.ndarray:
        self._require_trained()
        Xs = self.scaler.transform(X).astype(np.float32)
        return self._reconstruction_errors(Xs)

    def anomaly_score(self, X: np.ndarray) -> np.ndarray:
        errors = self.predict(X)
        if self.training_errors.size == 0:
            return errors
        sorted_train = np.sort(self.training_errors)
        ranks = np.searchsorted(sorted_train, errors, side="right")
        return ranks.astype(np.float32) / float(len(sorted_train) + 1)

    def is_anomaly(self, X: np.ndarray) -> np.ndarray:
        return self.predict(X) > self.threshold

    def _reconstruction_errors(self, Xs: np.ndarray) -> np.ndarray:
        device = torch.device(self.params.device)
        self.net.eval()
        with torch.no_grad():
            xt = torch.as_tensor(Xs, dtype=torch.float32, device=device)
            out = self.net(xt)
            errs = ((out - xt) ** 2).mean(dim=1).cpu().numpy()
        return errs

    def _eval_loss(self, loader: DataLoader, criterion: nn.Module, device: torch.device) -> float:
        self.net.eval()
        total = 0.0
        seen = 0
        with torch.no_grad():
            for (batch,) in loader:
                batch = batch.to(device)
                out = self.net(batch)
                loss = criterion(out, batch)
                total += float(loss.item()) * batch.size(0)
                seen += batch.size(0)
        return total / max(1, seen)

    def save(self, dir_path: str | Path) -> None:
        self._require_trained()
        dir_path = Path(dir_path)
        dir_path.mkdir(parents=True, exist_ok=True)
        torch.save(self.net.state_dict(), dir_path / "autoencoder.pt")
        joblib.dump({
            "scaler": self.scaler,
            "params": self.params,
            "threshold": self.threshold,
            "training_errors": self.training_errors,
        }, dir_path / "autoencoder_meta.joblib")

    def load(self, dir_path: str | Path) -> None:
        dir_path = Path(dir_path)
        meta = joblib.load(dir_path / "autoencoder_meta.joblib")
        self.scaler = meta["scaler"]
        self.params = meta.get("params", self.params)
        self.threshold = float(meta.get("threshold", 0.0))
        self.training_errors = meta.get("training_errors", np.zeros(0, dtype=np.float32))
        net = _Net(self.params.input_dim, self.params.hidden_dims)
        net.load_state_dict(torch.load(dir_path / "autoencoder.pt", map_location=self.params.device))
        net.eval()
        self.net = net.to(self.params.device)

    def _require_trained(self) -> None:
        if self.net is None or self.scaler is None:
            raise RuntimeError("autoencoder is not trained")
