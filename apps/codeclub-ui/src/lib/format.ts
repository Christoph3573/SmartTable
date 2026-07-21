export const formatDate = (date: string, options: Intl.DateTimeFormatOptions = { day: "2-digit", month: "short" }) =>
  new Intl.DateTimeFormat("de-DE", options).format(new Date(`${date.length === 10 ? `${date}T12:00:00` : date}`));
