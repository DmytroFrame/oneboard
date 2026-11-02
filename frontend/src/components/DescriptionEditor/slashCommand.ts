import { Extension, type Editor, type Range } from "@tiptap/core";
import { ReactRenderer } from "@tiptap/react";
import Suggestion, { type SuggestionProps } from "@tiptap/suggestion";
import { i18n } from "@/i18n";
import { SlashMenu, type MenuHandle } from "./SlashMenu";

// ---------- slash commands (type "/" for blocks, like Notion) ----------

export interface SlashItem {
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

export const SlashCommand = Extension.create({
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
