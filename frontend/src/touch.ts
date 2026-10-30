import { polyfill } from "mobile-drag-drop";
import "mobile-drag-drop/default.css";

/**
 * Touch screens don't fire HTML5 drag-and-drop events, which the board uses for moving cards.
 * The polyfill turns a long press + move into those same events, so dragging (and showing the drag
 * live to others) works on phones and tablets without a second implementation. A short swipe still
 * scrolls.
 */
export function initTouchDragAndDrop() {
  if (!matchMedia("(pointer: coarse)").matches) return;
  polyfill({
    forceApply: true, // iOS reports partial native support that doesn't work for this
    holdToDrag: 350,
    dragImageTranslateOverride: boardAutoScroll,
  });
  // iOS needs a non-passive touchmove listener for the polyfill to be able to prevent scrolling.
  window.addEventListener("touchmove", () => {}, { passive: false });
}

// ---------- edge auto-scroll while dragging ----------
// The polyfill's own helper scrolls the nearest scrollable ancestor of the finger, which on the board
// is a column's (vertical) card list — so it could never reach the next column. Here: near the left or
// right edge the row of columns scrolls sideways; near the top or bottom the column under the finger
// scrolls its cards. Speed grows as the finger gets closer to the edge.

const EDGE = 64; // px from an edge where scrolling starts
const MAX_SPEED = 22; // px per frame at the very edge

let point: { x: number; y: number } | null = null;
let translate: ((dx: number, dy: number) => void) | null = null;
let frame = 0;

const speed = (distance: number) => Math.ceil(MAX_SPEED * (1 - Math.max(0, distance) / EDGE));

function scrollStep(): boolean {
  if (!point) return false;
  let moved = false;

  const row = document.querySelector<HTMLElement>(".columns");
  if (row) {
    const r = row.getBoundingClientRect();
    if (point.x < r.left + EDGE && row.scrollLeft > 0) {
      row.scrollLeft -= speed(point.x - r.left);
      moved = true;
    } else if (point.x > r.right - EDGE && row.scrollLeft + row.clientWidth < row.scrollWidth - 1) {
      row.scrollLeft += speed(r.right - point.x);
      moved = true;
    }
  }

  for (const body of document.querySelectorAll<HTMLElement>(".column-body")) {
    const r = body.getBoundingClientRect();
    if (point.x < r.left || point.x > r.right) continue;
    if (point.y < r.top + EDGE && body.scrollTop > 0) {
      body.scrollTop -= speed(point.y - r.top);
      moved = true;
    } else if (point.y > r.bottom - EDGE && body.scrollTop + body.clientHeight < body.scrollHeight - 1) {
      body.scrollTop += speed(r.bottom - point.y);
      moved = true;
    }
    break;
  }
  return moved;
}

function tick() {
  frame = 0;
  if (scrollStep()) {
    translate?.(0, 0); // keep the drag image under the finger; the polyfill re-targets the drop on its own
    frame = requestAnimationFrame(tick);
  }
}

function boardAutoScroll(
  _event: TouchEvent | undefined,
  coordinates: { x: number; y: number } | undefined,
  _hovered: HTMLElement | undefined,
  translateDragImage: (dx: number, dy: number) => void,
) {
  if (!coordinates) {
    // The drag ended.
    point = null;
    if (frame) cancelAnimationFrame(frame);
    frame = 0;
    return;
  }
  point = coordinates;
  translate = translateDragImage;
  if (!frame) frame = requestAnimationFrame(tick);
}
