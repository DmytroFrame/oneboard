import { useRef, useState, type ChangeEvent } from "react";
import { api } from "../api";
import { useAuth } from "../auth";
import { Avatar } from "./Avatar";
import { Modal } from "./ProjectModals";

const AVATAR_SIZE = 256;
const MAX_SOURCE_BYTES = 20 << 20;

/** Center-crops the image to a square and scales it down, so uploads stay small whatever the camera produced. */
async function toSquareAvatar(file: File): Promise<Blob> {
  const bitmap = await createImageBitmap(file);
  const side = Math.min(bitmap.width, bitmap.height);
  const canvas = document.createElement("canvas");
  canvas.width = canvas.height = Math.min(AVATAR_SIZE, side);
  const ctx = canvas.getContext("2d")!;
  ctx.imageSmoothingQuality = "high";
  ctx.drawImage(bitmap, (bitmap.width - side) / 2, (bitmap.height - side) / 2, side, side, 0, 0, canvas.width, canvas.height);
  bitmap.close();
  const blob = await new Promise<Blob | null>((resolve) => canvas.toBlob(resolve, "image/webp", 0.9));
  // Browsers without WebP encoding fall back to PNG.
  return blob?.type === "image/webp" ? blob : new Promise((resolve) => canvas.toBlob((b) => resolve(b!), "image/png"));
}

export function ProfileModal({ onClose }: { onClose: () => void }) {
  const { user, setUser } = useAuth();
  const inputRef = useRef<HTMLInputElement>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function run(action: () => Promise<void>) {
    setBusy(true);
    setError("");
    try {
      await action();
    } catch (err) {
      setError((err as Error).message);
    } finally {
      setBusy(false);
    }
  }

  function onFile(e: ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = ""; // allow picking the same file again
    if (!file) return;
    if (!file.type.startsWith("image/")) return setError("Оберіть зображення");
    if (file.size > MAX_SOURCE_BYTES) return setError("Файл завеликий (до 20 МБ)");
    run(async () => {
      let image: Blob;
      try {
        image = await toSquareAvatar(file);
      } catch {
        throw new Error("Не вдалося прочитати зображення");
      }
      setUser(await api.uploadAvatar(image));
    });
  }

  if (!user) return null;
  return (
    <Modal title="Профіль" onClose={onClose}>
      <div className="modal-content profile">
        <div className={`profile-avatar${busy ? " profile-avatar--busy" : ""}`}>
          <Avatar user={user} size={112} />
          {busy && <span className="spinner" />}
        </div>
        <div className="profile-name">{user.name}</div>
        <div className="profile-email">{user.email}</div>

        <input ref={inputRef} type="file" accept="image/png,image/jpeg,image/webp,image/gif" hidden onChange={onFile} />
        <div className="profile-actions">
          <button className="btn btn-primary" disabled={busy} onClick={() => inputRef.current?.click()}>
            {user.avatarUrl ? "Змінити фото" : "Завантажити фото"}
          </button>
          {user.avatarUrl && (
            <button className="btn btn-ghost" disabled={busy} onClick={() => run(async () => setUser(await api.removeAvatar()))}>
              Видалити фото
            </button>
          )}
        </div>
        <p className="profile-hint">PNG, JPEG, WebP або GIF. Фото обріжеться до квадрата.</p>
        {error && <div className="form-error">{error}</div>}
      </div>
    </Modal>
  );
}
