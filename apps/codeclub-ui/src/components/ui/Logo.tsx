export function LogoMark({ size = 44 }: { size?: number }) {
  return <img src="/icon.png" alt="SmartTable Logo" width={size} height={size} className="rounded-xl object-contain" />;
}
