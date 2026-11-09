import ReservationsPageClient from "@/components/reservations/ReservationsPageClient";
import { isDay } from "@/lib/reservations/reservations";

interface ReservationsPageProps {
  searchParams: Promise<{ date?: string }>;
}

export default async function ReservationsPage({ searchParams }: ReservationsPageProps) {
  const { date } = await searchParams;
  return <ReservationsPageClient requestedDay={isDay(date) ? date : null} />;
}
