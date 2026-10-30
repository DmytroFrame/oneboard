import { Extension, type Editor, type Range } from "@tiptap/core";
import Collaboration from "@tiptap/extension-collaboration";
import CollaborationCaret from "@tiptap/extension-collaboration-caret";
import { TaskItem, TaskList } from "@tiptap/extension-list";
import { Placeholder } from "@tiptap/extensions";
import { Markdown } from "@tiptap/markdown";
import { EditorContent, ReactRenderer, useEditor } from "@tiptap/react";
import { BubbleMenu } from "@tiptap/react/menus";
import StarterKit from "@tiptap/starter-kit";
import Suggestion, { type SuggestionProps } from "@tiptap/suggestion";
import { forwardRef, useEffect, useImperativeHandle, useState } from "react";
import { useTranslation } from "react-i18next";
import type { CardDocProvider } from "../cardDoc";
import { i18n } from "../i18n";

// ---------- slash commands (type "/" for blocks, like Notion) ----------

interface SlashItem {
  id: "text" | "h1" | "h2" | "h3" | "bullet" | "ordered" | "todo" | "quote" | "code" | "divider";
  icon: string;
  keywords: string;
  run: (editor: Editor, range: Range) => void;
}

const chain = (editor: Editor, range: Range) => editor.chain().focus().deleteRange(range);

const SLASH_ITEMS: SlashItem[] = [
  { id: "text", icon: "¶", keywords: "text paragraph", run: (e, r) => chain(e, r).setParagraph().run() },
  { id: "h1", icon: "H1", keywords: "heading title h1", run: (e, r) => chain(e, r).setHeading({ level: 1 }).run() },
  { id: "h2", icon: "H2", keywords: "heading subtitle h2", run: (e, r) => chain(e, r).setHeading({ level: 2 }).run() },
  { id: "h3", icon: "H3", keywords: "heading h3", run: (e, r) => chain(e, r).setHeading({ level: 3 }).run() },
  { id: "bullet", icon: "•", keywords: "bullet list unordered", run: (e, r) => chain(e, r).toggleBulletList().run() },
  { id: "ordered", icon: "1.", keywords: "numbered ordered list", run: (e, r) => chain(e, r).toggleOrderedList().run() },
  { id: "todo", icon: "☑", keywords: "todo task checkbox check", run: (e, r) => chain(e, r).toggleTaskList().run() },
  { id: "quote", icon: "❝", keywords: "quote blockquote", run: (e, r) => chain(e, r).toggleBlockquote().run() },
  { id: "code", icon: "</>", keywords: "code block snippet", run: (e, r) => chain(e, r).toggleCodeBlock().run() },
  { id: "divider", icon: "—", keywords: "divider rule line hr", run: (e, r) => chain(e, r).setHorizontalRule().run() },
];

const label = (item: SlashItem) => i18n.t(`editor.blocks.${item.id}`);

interface MenuHandle {
  onKeyDown: (e: KeyboardEvent) => boolean;
}

const SlashMenu = forwardRef<MenuHandle, SuggestionProps<SlashItem>>(function SlashMenu({ items, command }, ref) {
  const { t } = useTranslation();
  const [index, setIndex] = useState(0);
  useEffect(() => setIndex(0), [items]);

  useImperativeHandle(ref, () => ({
    onKeyDown: (e) => {
      if (!items.length) return false;
      if (e.key === "ArrowDown") setIndex((i) => (i + 1) % items.length);
      else if (e.key === "ArrowUp") setIndex((i) => (i + items.length - 1) % items.length);
      else if (e.key === "Enter") command(items[index]!);
      else return false;
      return true;
    },
  }));

  if (!items.length) return <div className="slash-menu slash-menu--empty">{t("editor.noMatches")}</div>;
  return (
    <div className="slash-menu" role="listbox">
      {items.map((item, i) => (
        <button
          key={item.id}
          type="button"
          role="option"
          aria-selected={i === index}
          className={`slash-item${i === index ? " slash-item--active" : ""}`}
          onMouseEnter={() => setIndex(i)}
          onMouseDown={(e) => (e.preventDefault(), command(item))}
        >
          <span className="slash-icon">{item.icon}</span>
          <span className="slash-text">
            <span>{t(`editor.blocks.${item.id}`)}</span>
            <small>{t(`editor.hints.${item.id}`)}</small>
          </span>
        </button>
      ))}
    </div>
  );
});

const SlashCommand = Extension.create({
  name: "slashCommand",
  addProseMirrorPlugins() {
    return [
      Suggestion<SlashItem>({
        editor: this.editor,
        char: "/",
        items: ({ query }) => {
          const q = query.toLowerCase();
          return SLASH_ITEMS.filter((it) => !q || it.keywords.includes(q) || label(it).toLowerCase().includes(q));
        },
        command: ({ editor, range, props }) => props.run(editor, range),
        render: () => {
          let renderer: ReactRenderer<MenuHandle, SuggestionProps<SlashItem>> | null = null;
          const place = (props: SuggestionProps<SlashItem>) => {
            const rect = props.clientRect?.();
            const el = renderer?.element as HTMLElement | undefined;
            if (!rect || !el) return;
            el.style.position = "fixed";
            el.style.left = `${rect.left}px`;
            el.style.top = `${rect.bottom + 6}px`;
            el.style.zIndex = "300";
          };
          return {
            onStart: (props) => {
              renderer = new ReactRenderer(SlashMenu, { props, editor: props.editor });
              document.body.appendChild(renderer.element);
              place(props);
            },
            onUpdate: (props) => {
              renderer?.updateProps(props);
              place(props);
            },
            onKeyDown: ({ event }) => {
              if (event.key === "Escape") return false;
              return renderer?.ref?.onKeyDown(event) ?? false;
            },
            onExit: () => {
              renderer?.element.remove();
              renderer?.destroy();
              renderer = null;
            },
          };
        },
      }),
    ];
  },
});

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
      editorProps: { attributes: { class: "md-editor-content" } },
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
    <button type="button" className={editor.isActive(name) ? "is-active" : ""} onMouseDown={(e) => (e.preventDefault(), run())} title={title}>
      {text}
    </button>
  );

  return (
    <div className="md-editor">
      <BubbleMenu editor={editor} className="bubble-menu">
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
