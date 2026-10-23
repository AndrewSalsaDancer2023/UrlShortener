Управление памятью в контейнерах STL. От абстракций к реализации

Каждый разработчик на C++ начинает свой путь с оператора new. Нам объясняют его работу очень просто: "new выделяет память в куче и создает там объект". Мы пишем MyClass* ptr = new MyClass(42); и до поры до времени не задумываемся, что скрывается за этой строчкой.

Между тем, в ней жестко связываются два абсолютно разных процесса:

Выделение памяти в куче.

Инициализация объекта путём вызова конструктора с передачей аргументов.

Для одиночных объектов это удобно. Но как только мы переходим к использованию std::vector это связывание становится источником проблем. Представьте, что вы хотите создать вектор емкостью 100 элементов, но прямо сейчас в нем лежит всего 2 объекта. Если бы std::vector выделял память через new MyClass[100], то нужно было бы сразу вызывать конструкторы по умолчанию для всех 100 элементов. А если создание объекта — это "тяжелая" операция, включающая сетевые запросы? Зачем тратить процессорное время на создание ненужных в данный момент 98 объектов?

Контейнерам STL жизненно необходим другой подход: возможность отделить выделение памяти от создания объектов. Выделить память под 100 элементов, сконструировать два объекта, а память под остальные 98 оставить в резерве на будущее.

Для этого в языке существует механизм, разделяющий операцию создания на два независимых этапа:

За выделение памяти отвечает функция operator new(size_t). Она запрашивает блок памяти нужного размера и возвращает указатель типа void*.

За создание объекта в выделенной памяти отвечает размещающий new (placement new). Его синтаксис выглядит так: ::new (storage_ptr) MyClass(args...). Он берет указатель на выделенную память storage_ptr и вызывает конструктор класса.

Казалось бы, вот оно — идеальное решение! Мы можем разделить выделение памяти и создание объекта путем вызова operator new и placement new внутри нашего контейнера.

Но архитектура STL создавалась с прицелом на максимальную гибкость. Что, если пользователь не хочет выделять память из стандартной системной кучи? Что, если к программе предъявляются жесткие требования по быстродействию, где стандартный распределитель работает слишком медленно, и память нужно брать из заранее выделенного буфера?

Если мы жестко зашьем вызовы new внутрь кода контейнера, скажем, std::vector, то лишим разработчиков возможности кастомизировать управление памятью. Именно для решения этой проблемы в стандартную библиотеку были введены аллокатор (std::allocator) и вспомогательный класс — std::allocator_traits.

Давайте разберемся, как эти абстракции взаимодействуют между собой, как заставить их работать вместе, и как все-таки выделяется память для объектов. Так как интерфейс аллокаторов и их внутренние механизмы работы менялись со времен С++98 и материала довольно много, то сегодня рассмотрим философию, заложенную в C++11, но пример реализации напишем на C++20, чтобы не углубляться в шаблонный код.

Как работает стандартный аллокатор std::allocator

По умолчанию контейнеры STL (например, std::vector<int> или std::list<int>) используют аллокатор в качестве аргумента, принимающего значение по умолчанию std::allocator<T>. Его задача — отделить выделение физической памяти от конструирования объектов при помощи двух методов:

allocate(n). Выделяет блок памяти, достаточный для хранения n объектов типа T. Никакие конструкторы при этом не вызываются. Под капотом для этого используется стандартная системная функция operator new(size_t).

construct(p, args...). Создает объект типа T в выделенной не инициализированной области памяти, на которую указывает p, используя глобальный placement-new: ::new(p) T(std::forward<Args>(args)...).

При удалении происходит обратный процесс:

destroy(p): Вызывает деструктор объекта, на который указывает p.

deallocate (p, n): Освобождает блок памяти, ранее выделенный с помощью allocate с помощью ::operator delete.

Код минимального аллокатора выглядит так:

#include <cstddef>
#include <iostream>
#include <memory>
#include <vector>

template <class T>
struct custom_allocator {
  typedef T value_type;
  custom_allocator() noexcept {}
  // Контейнер запрашивает память под n элементов типа T
  T* allocate (std::size_t n)
  {
    if (n == 0) return nullptr;
    // Вычисляем точный размер в байтах
    // Вызываем оператор new
    // Никакие конструкторы T() здесь НЕ вызываются
    return static_cast<T*>(::operator new(n*sizeof(T))); 
  }
  
  void deallocate (T* p, std::size_t n)
  { 
    if (p == nullptr) return;
    // Освобождаем память через глобальный оператор delete
    // Деструкторы элементов к этому моменту уже ДОЛЖНЫ быть вызваны контейнером
    ::operator delete(p);
  }
};
Всё что нужно — это определить один тип (value_type) и пару функций: allocate, deallocate.

Думаю вы обратили внимание на функции operator new и ::operator delete. operator new просто выделяет память, ничего не зная о том, что вы хотите в ней разместить (конструктор объекта не вызывается). ::operator delete работает в паре с ::operator new и отвечает за освобождение памяти.

При работе с ::operator new и ::operator delete конструкторы и деструкторы нужно вызывать вручную. Например, мы хотим использовать простой класс Monster.

class Monster {
private:
    std::string name;
    int hp;

public:
    Monster(std::string monsterName, int monsterHp)
        : name(monsterName), hp(monsterHp) {
        std::cout << "Constructor Monster " << name << " created with " << hp << " HP.\n";
    }
    virtual void Attack() {/* some attack code */}
    ~Monster() {
        std::cout << "Destructor Monster " << name << " is destroyed.\n";
    }
};
Вначале вызываем operator new, затем конструктор. После того, как работа с объектом завершена нужно вначале вызвать деструктор и operator delete.

// ШАГ 1: Выделение памяти с помощью ::operator new
// Выделяем ровно столько байт, сколько занимает объект класса Monster
void* rawMemory = ::operator new(sizeof(Monster));
std::cout << "Memory Allocated " << sizeof(Monster) << " bytes of raw memory.\n\n";

// ШАГ 2: Вызов конструктора объекта с помощью размещения (placement new)
// Мы передаем указатель на блок памяти
// Память повторно не выделяется, там конструируется объект
Monster* myMonster = new (rawMemory) Monster("Shambler", 500);
std::cout << "Object is now fully constructed.\n\n";

myMonster->Attack();
// ШАГ 3: Ручной вызов деструктора
// Так как мы сами управляем жизненным циклом, мы обязаны вручную вызвать деструктор,
// чтобы объект корректно освободил свои внутренние ресурсы
myMonster->~Monster();
std::cout << "Object resources are cleaned up. Memory is now 'raw' again.\n\n";

// ШАГ 4: Освобождение памяти с помощью глобального ::operator delete
// Возвращаем память операционной системе. Передаем указатель на начало выделенного блока
::operator delete(rawMemory);
Важные правила при ручном выделении и освобождении памяти:

Нельзя вызывать delete myMonster. Если в конце этого примера вы случайно напишете delete myMonster, то программа попытается вызвать деструктор второй раз и освободить память стандартным путем. Это приведет к ошибке повторного освобождения памяти и аварийному завершению программы.

Порядок имеет значение. Сначала вы обязаны вызвать деструктор ~Monster(), и только после этого освободить память через ::operator delete. Если вы сначала освободите память, вызов деструктора попытается обратиться к уже несуществующим данным, вызвав неопределенное поведение.

Если необходимо сравнивать контейнеры, в которые передан пользовательский аллокатор, то необходимо определить operator !=. В рассматриваемом простейшем случае аллокатор не имеет состояния, то есть внутренних полей данных, поэтому оператор сравнения выглядит тривиально:

template <class T, class U>
constexpr bool operator!= (const custom_allocator<T>&, const custom_allocator<U>&) noexcept
{return false;}
На самом деле для правильного функционирования аллокатора необходимо объявить гораздо больше типов и методов. Начиная с С++11, контейнеры вродеstd::vector и std::list никогда не вызывают методы аллокатора напрямую. Вместо этого они всегда работают через std::allocator_traits<Allocator>.

В задачи, решаемые std::allocator_traits входят:

Добавление необходимых типов данных. Cтандарт C++98 требовал, чтобы пользовательский аллокатор содержал большое количество вспомогательных типов данных (pointer, const_pointer, difference_type и т.д.). В С++11, если вы не определили их в своем аллокаторе, std::allocator_traits автоматически добавит их.

Управление жизненным циклом объектов. Предоставляет методы construct для вызова конструктора элемента и destroy() для вызова деструктора. Если в вашем аллокаторе нет этих методов, std::allocator_traits сам вызовет placement new и деструктор. Доступ к методам аллокатора allocate, deallocate, construct, destroy осуществляется через статические функции-обертки.

В C++ аллокатор является частью типа шаблона контейнера. Объявление вектора выглядит как template < class T, class Alloc = allocator<T> > class vector. Если две переменных типа std::vector инстанцируются с различными типами Alloc, то компилятор создает два совершенно разных класса. Поэтому переменная std::vector<int> v1 (которая неявно означает std::vector<int, std::allocator<int>> v1) и переменная std::vector<int,custom_allocator<int>> v2 имеют разные типы с точки зрения компилятора.

Также вы не можете написать if (v1 == v2), потому что для std::vector оператор == существует только для векторов с одинаковыми аллокаторами:

template< class T, class Alloc >
bool operator==( const std::vector<T,Alloc>& lhs, const std::vector<T,Alloc>& rhs )
Если функция ожидает вектор с дефолтным аллокатором, то вы не сможете передать туда вектор с пользовательским распределителем, например:

void print_vector(const std::vector<int>& vec)
{ 
  for(const auto& el : vec)
    std::cout << el << ", "; 
}

std::vector<int, custom_allocator<int>> foo{1, 2, 3}; // Дефолтный аллокатор

print_vector(foo); // ошибка компиляции!
Чтобы решить эту проблему можно заставить print_vector принимать аллокатор как параметр шаблона. Тогда компилятор создаст версию функции для каждого типа вектора.

template <typename Alloc>
// Работает как с std::allocator, так и с custom_allocator
void print_vector(const std::vector<int, Alloc>& vec) 
{
   for(const auto& el : vec)
     std::cout << el << ", "; 
}
Чтобы окончательно разобраться с тем, как работает связка allocator, allocator_traits, контейнер напишем минималистичную собственную версию allocator_traits, которая будет использована для выделения памяти в вектор-подобном контейнере. allocator_traits в библиотеке stl написан с использованием шаблонов, но мы используем концепты, чтобы максимально упростить логику.

template<typename Alloc, typename T, typename... Args>
concept HasConstruct = requires(Alloc& a, T* p, Args&&... args) {
    a.construct(p, std::forward<Args>(args)...);
};

template<typename Alloc, typename T>
concept HasDestroy = requires(Alloc& a, T* p) {
    a.destroy(p);
};

template<typename Alloc>
struct my_allocator_traits {
    using value_type      = typename Alloc::value_type;
    using pointer         = value_type*;
    using size_type        = std::size_t;

    // Выделение памяти делегируем аллокатору
    static pointer allocate(Alloc& a, size_type n) {
        return a.allocate(n);
    }

    // Освобождение памяти делегируем аллокатору
    static void deallocate(Alloc& a, pointer p, size_type n) {
        a.deallocate(p, n);
    }

    // Конструирование: концепты + if constexpr
    template<typename T, typename... Args>
    static void construct(Alloc& a, T* p, Args&&... args) {
        if constexpr (HasConstruct<Alloc, T, Args...>) {
            a.construct(p, std::forward<Args>(args)...);
        } else {
            std::cout << "my_allocator_traits calls placement new \n";
            ::new(static_cast<void*>(p)) T(std::forward<Args>(args)...);
        }
    }

    // Уничтожение: концепты + if constexpr
    template<typename T>
    static void destroy(Alloc& a, T* p) {
        if constexpr (HasDestroy<Alloc, T>) {
            a.destroy(p);
        } else {
            std::cout << "my_allocator_traits calls object destructor \n";
            p->~T();
        }
    }
};
Теперь напишем упрощенный до предела вектор-подобный контейнер:

template<typename T, typename Alloc = std::allocator<T>>
class SimpleVector {
private:
    using Traits = my_allocator_traits<Alloc>;
    
    Alloc m_alloc;       
    T* m_data = nullptr; 
    size_t m_cap = 0;    // вместимость
    size_t m_size = 0;   // количество хранимых объектов

    // Вспомогательный метод для изменения размера памяти
    void reallocate(size_t new_cap) {
        T* new_data = Traits::allocate(m_alloc, new_cap);

        // Переносим старые объекты в новую память
        for (size_t i = 0; i < m_size; ++i) {
            Traits::construct(m_alloc, new_data + i, std::move(m_data[i]));
            Traits::destroy(m_alloc, m_data + i);
        }

        // Освобождаем старую память
        if (m_data) {
            Traits::deallocate(m_alloc, m_data, m_cap);
        }

        m_data = new_data;
        m_cap = new_cap;
    }

public:
    SimpleVector() = default;

    ~SimpleVector() {
        clear();
        if (m_data) {
            Traits::deallocate(m_alloc, m_data, m_cap);
        }
    }

    void push_back(const T& value) {
        if (m_size >= m_cap) {
            reallocate(m_cap == 0 ? 1 : m_cap * 2);
        }
        // Вызываем конструирование через traits
        Traits::construct(m_alloc, m_data + m_size, value);
        m_size++;
    }

    void pop_back() {
        if (m_size == 0) return;
        m_size--;
        // Вызываем уничтожение через traits
        Traits::destroy(m_alloc, m_data + m_size);
    }

    void clear() {
        while (m_size > 0) {
            pop_back();
        }
    }

    size_t size() const { return m_size; }
    T& operator[](size_t index) { return m_data[index]; }
};
Теперь создадим аллокатор, в котором есть методы construct и destroy.

template<typename T>
struct verbose_allocator {
    using value_type = T;
    T* allocate(size_t n) { return static_cast<T*>(::operator new(n * sizeof(T))); }
    void deallocate(T* p, size_t) { ::operator delete(p); }

    template<typename... Args>
    void construct(T* p, Args&&... args) {
        std::cout << "Verbose Allocator construct \n";
        ::new((void*)p) T(std::forward<Args>(args)...);
    }

    void destroy(T* p) {
        std::cout << "Verbose Allocator destroy \n";
        p->~T();
    }
};
И проверим наш контейнер, задавая созданный ранее custom_allocator и verbose_allocator.

// Тестовый класс, который сообщает о своем создании
struct Item {
    int id;
    Item(int i) : id(i) { std::cout << "Item with id=" << id << " created \n"; }
    ~Item() { std::cout << "Item with id=" << id << " destroyed \n"; }
};

int main() {
    std::cout << "1. Test with custom_allocator placement new called \n";
    {
        SimpleVector<Item, custom_allocator<Item>> vec;
        vec.push_back(Item(1));
        std::cout << "Deleting element from vector \n";
        vec.pop_back();
    }

    std::cout << "\n 2. Test with verbose_allocator (vector calls allocator methods) \n";
    {
        SimpleVector<Item, verbose_allocator<Item>> vec;
        vec.push_back(Item(2));
        std::cout << "Deleting element from vector \n";
        vec.pop_back();
    }

    return 0;
}
Результат работы программы:

1. Test with custom_allocator placement new called 
Item with id=1 created 
my_allocator_traits calls placement new 
Item with id=1 destroyed 
Deleting element from vector 
my_allocator_traits calls object destructor 
Item with id=1 destroyed 

 2. Test with verbose_allocator (vector calls allocator methods) 
Item with id=2 created 
Verbose Allocator construct 
Item with id=2 destroyed 
Deleting element from vector 
Verbose Allocator destroy 
Item with id=2 destroyed 
Для полного просветления спустимся еще на один уровень ниже и посмотрим как устроен operator new под капотом.

_GLIBCXX_WEAK_DEFINITION void *
operator new (std::size_t sz) _GLIBCXX_THROW (std::bad_alloc)
{
  void *p;

  // new должен выделить хотя бы один байт
  if (__builtin_expect (sz == 0, false))
    sz = 1;
  // попытка выделить память через вызов функции malloc языка Си
  while ((p = malloc (sz)) == 0)
    {
      // если память выделить не удалось, то проверяем, 
      // установлен ли пользовательский обработчик нехватки памяти (new_handler)
      new_handler handler = std::get_new_handler ();
      if (! handler) // если обработчика нет, выбрасываем исключение 
        _GLIBCXX_THROW_OR_ABORT(bad_alloc());
      // иначе вызываем обработчик в цикле пока не сможем выделить память
      handler ();
    }

  return p;
}
Таким образом, ::operator new — это высоко уровневая C++ обертка над функцией malloc языка Си. Главное отличие от malloc заключается в том, что new умеет вызывать обработчик нехватки памяти и генерирует стандартное исключение std::bad_alloc, если обработчика нет, вместо возврата nullptr. То есть напрямую память у ОС ::operator new не запрашивает.

На мой взгляд, функция malloc — это один из самых сложных компонентов стандартной библиотеки языка Си. Постараюсь описать её работу в общих чертах.

Malloc выделяет памяти больше чем запрошено. Память нарезается на чанки (блоки). Каждый чанк содержит метаданные, которые идут перед тем адресом, который возвращается malloc.

Запрашивать память у ядра ОС при каждом вызове непозволительно медленно. Поэтому malloc удерживает освобожденную память в пуле и сортирует чанки по специальным спискам — корзинам (bins). Насколько я понимаю, существует множество таких корзин для быстрого поиска нужного размера.

Существуют корзины для малых (менее 128 байт на 64 битных системах), средних и больших объемов памяти. Ещё есть несортированная корзина. Сюда временно попадают все освобожденные чанки. Malloc просматривает её и пытается склеить соседние свободные чанки вместе, чтобы избежать фрагментации памяти.

При запросе выделения памяти вначале идёт поиск в корзинах. Если подходящий объем найти не удалось, то происходит обращение к ОС. В Unix-подобных ОС если запрашивается память меньше некоторого заданного порога, то malloc увеличивает размер сегмента данных процесса через системный вызов brk(). Если большой кусок, например, несколько мегабайт, то malloc делает системный вызов mmap().

Таким образом, malloc представляет универсальный механизм, который может выделять блоки памяти разных размеров. Для этого приходится выполнять поиск по ряду внутренних структур данных, что замедляет работу.

В этой статье мы детально разобрали, как устроена работа с памятью в контейнерах C++, почему аллокатор использует allocator_traits и как выделяется память на уровне ОС.

Malloc — это универсальная функция, но у универсальности есть обратная сторона — скорость. Какими бы быстрыми ни были современные системные аллокаторы, они всё равно тратят драгоценное время на поиск свободных чанков, что сказывается на скорости работы программы.

В следующей статье перейдем от теории к практике и займемся оптимизацией. Мы напишем собственные кастомные аллокаторы, которые могут значительно ускорить ваш код.

