export function StatCard({
  label,
  value,
  subtitle,
}: {
  label: string;
  value: string | number;
  subtitle?: string;
}) {
  return (
    <div className="rounded-lg bg-[#1f1f1f] border border-neutral-800 p-5">
      <p className="text-xs uppercase tracking-wider text-neutral-500 mb-1">
        {label}
      </p>
      <p className="text-3xl font-bold text-white">{value}</p>
      {subtitle && (
        <p className="text-sm text-neutral-400 mt-1">{subtitle}</p>
      )}
    </div>
  );
}
