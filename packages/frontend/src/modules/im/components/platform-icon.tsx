import feishu from "../assets/feishu.png";
import wechat from "../assets/wechat.svg";

export function PlatformIcon({
  platform,
  compact = false,
}: {
  platform?: string;
  compact?: boolean;
}) {
  return (
    <span
      className={
        compact
          ? "size-icon-md grid shrink-0 place-items-center"
          : "size-avatar-md grid shrink-0 place-items-center"
      }
      aria-hidden="true"
    >
      <img
        src={platform === "feishu" ? feishu : wechat}
        alt=""
        className={
          compact
            ? "size-icon-md object-contain"
            : "size-avatar-sm object-contain"
        }
      />
    </span>
  );
}
