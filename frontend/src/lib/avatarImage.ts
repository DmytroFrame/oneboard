const AVATAR_SIZE = 256;

/** Center-crops the image to a square and scales it down, so uploads stay small whatever the camera produced. */
export async function toSquareAvatar(file: File): Promise<Blob> {
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
