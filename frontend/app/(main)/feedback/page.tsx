import FeedbackPageClient from "@/components/feedback/FeedbackPageClient";
import { isDay } from "@/lib/reservations/reservations";

interface FeedbackPageProps {
  searchParams: Promise<{ date?: string; period?: string }>;
}

export default async function FeedbackPage({ searchParams }: FeedbackPageProps) {
  const { date, period } = await searchParams;
  return (
    <FeedbackPageClient
      requestedDay={isDay(date) ? date : null}
      requestedPeriod={period === "month" ? "month" : "week"}
    />
  );
}
