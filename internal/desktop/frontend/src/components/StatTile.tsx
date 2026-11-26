type Props = {
  label: string;
  value: string;
};

export function StatTile({ label, value }: Props) {
  return (
    <div className="stats-tile">
      <small>{label}</small>
      <b>{value}</b>
    </div>
  );
}