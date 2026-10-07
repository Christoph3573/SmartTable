import { useMemo } from "react";
import { useQuery } from "@tanstack/react-query";
import { coursesApi, smartTableCourseKey, type Course } from "../api/courses";

/**
 * Kurs-Sichtbarkeit (Schule): liefert die abgewählten Kurse aus dem
 * Kurse-Reiter. Fehlt eine Kurs-Zeile, gilt der Kurs als sichtbar (Opt-out).
 * Eigene Kurse (provider=custom) werden als zusätzliche Stunden eingeblendet.
 */
export function useCourseVisibility() {
  const { data } = useQuery({ queryKey: ["courses"], queryFn: coursesApi.list });

  return useMemo(() => {
    const courses = data ?? [];
    const hidden = new Set<string>();
    const custom: Course[] = [];
    for (const course of courses) {
      if (course.provider === "custom") {
        if (course.selected) custom.push(course);
        continue;
      }
      if (!course.selected) hidden.add(`${course.provider}:${course.external_key}`);
    }
    return {
      loading: false,
      hidden,
      custom,
      /** SmartTable-Fach (Subject) ausgeblendet? */
      hiddenSubject: (subjectId: number) => hidden.has(`smarttable:${smartTableCourseKey(subjectId)}`),
      /** SchoolConnect-Kurskürzel ausgeblendet? */
      hiddenKurs: (kurs: string) => hidden.has(`schoolconnect:${kurs}`),
    };
  }, [data]);
}
