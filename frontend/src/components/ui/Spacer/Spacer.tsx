import s from "./Spacer.module.scss";

/** Takes the free space in a flex row, pushing what follows to the end. */
export function Spacer() {
  return <div className={s.spacer} />;
}
