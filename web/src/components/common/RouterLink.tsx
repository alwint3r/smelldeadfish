import { Link } from "preact-router";
import type { JSX } from "preact";

type RouterLinkProps = JSX.AnchorHTMLAttributes<HTMLAnchorElement>;

export function RouterLink(props: RouterLinkProps) {
  return <Link {...(props as unknown as JSX.HTMLAttributes<HTMLAnchorElement>)} />;
}
