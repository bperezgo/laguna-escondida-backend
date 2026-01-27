import { serverApiRequest } from "@/lib/api/server";
import type { Expense, ExpenseCategory, ExpenseListResponse, ExpenseCategoryListResponse } from "@/types/expense";
import type { Supplier, SupplierListResponse } from "@/types/supplier";
import ExpensesPageClient from "@/components/expenses/ExpensesPageClient";

export default async function ExpensesPage() {
  let expenses: Expense[] = [];
  let categories: ExpenseCategory[] = [];
  let suppliers: Supplier[] = [];

  try {
    const [expensesRes, categoriesRes, suppliersRes] = await Promise.all([
      serverApiRequest<ExpenseListResponse>("/expenses"),
      serverApiRequest<ExpenseCategoryListResponse>("/expense-categories"),
      serverApiRequest<SupplierListResponse>("/suppliers"),
    ]);

    const expensesData = await expensesRes.json();
    const categoriesData = await categoriesRes.json();
    const suppliersData = await suppliersRes.json();

    expenses = expensesData.expenses || [];
    categories = categoriesData.categories || [];
    suppliers = suppliersData.suppliers || [];
  } catch (error) {
    console.error("Error fetching expenses data:", error);
  }

  return (
    <ExpensesPageClient
      initialExpenses={expenses}
      initialCategories={categories}
      initialSuppliers={suppliers}
    />
  );
}
