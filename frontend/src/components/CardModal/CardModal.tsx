import { useEffect } from "react";
import { ModalBackdrop } from "@/components/ui/Modal/Modal";
import type { BoardSocket } from "@/lib/realtime";
import type { Attachment, Card, CardInput, Column, Tag, User } from "@/types";
import type { Planning } from "@/types/tasks";
import { ExistingCard } from "./ExistingCard";
import { NewCard } from "./NewCard";

export interface CardModalProps {
  card: Card | null; // null = creating a new card
  initialColumnId: number;
  columns: Column[];
  users: User[];
  currentUser: User;
  canDelete: boolean;
  isProjectOwner: boolean;
  socket: BoardSocket | null;
  peersHere: User[]; // other people who have this card open right now
  onClose: () => void;
  onAttachmentsChange: (card: Card, attachments: Attachment[]) => void;
  onCreate: (input: CardInput, columnId: number) => Promise<void>;
  onUpdate: (card: Card, input: CardInput) => Promise<void>;
  onMove: (card: Card, columnId: number) => Promise<void>;
  onDelete: (card: Card) => Promise<void>;
  // OmiTeam task management
  planning: Planning;
  tags: Tag[];
  /** A change made outside the shared text (details, checklists, comments): refresh the card face. */
  onCardChanged: (card: Card) => void;
  /** A card appeared (created from a template) or left the board (turned into a template). */
  onCardAdded: (card: Card) => void;
  onCardRemoved: (card: Card) => void;
}

/** The task editor: a form for a new card, a live shared editor for an existing one. Lazily loaded. */
export function CardModal(props: CardModalProps) {
  const { onClose } = props;
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      // Escape first closes the editor's slash menu, then the modal.
      if (e.key === "Escape" && !document.querySelector("[data-slash-menu]")) onClose();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);

  return <ModalBackdrop onClose={onClose}>{props.card ? <ExistingCard {...props} card={props.card} /> : <NewCard {...props} />}</ModalBackdrop>;
}
