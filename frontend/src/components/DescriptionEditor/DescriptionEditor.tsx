import type { Editor } from "@tiptap/core";
import Collaboration from "@tiptap/extension-collaboration";
import CollaborationCaret from "@tiptap/extension-collaboration-caret";
import { TaskItem, TaskList } from "@tiptap/extension-list";
import { Placeholder } from "@tiptap/extensions";
import { Markdown } from "@tiptap/markdown";
import { EditorContent, useEditor } from "@tiptap/react";
import { BubbleMenu } from "@tiptap/react/menus";
import StarterKit from "@tiptap/starter-kit";
import { useEffect } from "react";
import { useTranslation } from "react-i18next";
import { i18n } from "@/i18n";
import type { CardDocProvider } from "@/lib/cardDoc";
import { cx } from "@/lib/cx";
import { SlashCommand } from "./slashCommand";
import s from "./DescriptionEditor.module.scss";

// ---------- the editor ----------

interface Props {
  /** Collaborative mode: bound to the card's shared document. */
  provider?: CardDocProvider;
  user: { name: string; color: string };
  /** Local mode (a card that doesn't exist yet): initial markdown and change callback. */
  initialMarkdown?: string;
  onChange?: (markdown: string) => void;
  editable?: boolean;
  onReady?: (editor: Editor) => void;
}

export function DescriptionEditor({ provider, user, initialMarkdown, onChange, editable = true, onReady }: Props) {
  const { t } = useTranslation();
  const editor = useEditor(
    {
      extensions: [
        StarterKit.configure({ undoRedo: provider ? false : undefined, link: { openOnClick: false } }),
        TaskList,
        TaskItem.configure({ nested: true }),
        Markdown,
        Placeholder.configure({ placeholder: () => i18n.t("editor.placeholder") }),
        SlashCommand,
        ...(provider
          ? [
              Collaboration.configure({ document: provider.doc, field: "description" }),
              CollaborationCaret.configure({ provider, user }),
            ]
          : []),
      ],
      content: provider ? undefined : (initialMarkdown ?? ""),
      contentType: provider ? undefined : "markdown",
      editable,
      onUpdate: ({ editor }) => onChange?.(editor.getMarkdown()),
      editorProps: { attributes: { class: s.content } },
    },
    [provider],
  );

  useEffect(() => {
    if (editor) onReady?.(editor);
  }, [editor, onReady]);

  useEffect(() => {
    editor?.setEditable(editable);
  }, [editor, editable]);

  if (!editor) return null;

  const mark = (name: string, run: () => void, text: string, title: string) => (
    <button type="button" className={cx(editor.isActive(name) && s.active)} onMouseDown={(e) => (e.preventDefault(), run())} title={title}>
      {text}
    </button>
  );

  return (
    <div className={s.editor}>
      <BubbleMenu editor={editor} className={s.bubble}>
        {mark("bold", () => editor.chain().focus().toggleBold().run(), "B", t("editor.bold"))}
        {mark("italic", () => editor.chain().focus().toggleItalic().run(), "I", t("editor.italic"))}
        {mark("strike", () => editor.chain().focus().toggleStrike().run(), "S", t("editor.strike"))}
        {mark("code", () => editor.chain().focus().toggleCode().run(), "</>", t("editor.code"))}
        {mark(
          "link",
          () => {
            if (editor.isActive("link")) return editor.chain().focus().unsetLink().run();
            const href = prompt(t("editor.linkPrompt"), "https://");
            if (href && /^https?:\/\//i.test(href)) editor.chain().focus().setLink({ href }).run();
          },
          "↗",
          t("editor.link"),
        )}
      </BubbleMenu>
      <EditorContent editor={editor} />
    </div>
  );
}
